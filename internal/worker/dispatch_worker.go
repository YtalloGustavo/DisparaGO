package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"disparago/internal/config"
	"disparago/internal/evolutiongo"
	"disparago/internal/queue"
	"disparago/internal/repository"
	"disparago/internal/service"
)

type DispatchWorker struct {
	log        *log.Logger
	repository *repository.CampaignRepository
	settings   *service.InstanceSettingsService
	redisQueue queue.Consumer
	provider   *evolutiongo.Client
	humanizer  config.HumanizerConfig
	retry      config.RetryConfig
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
	lanes      map[string]*dispatchLane
}

type dispatchLane struct {
	mu             sync.Mutex
	lastDispatchAt time.Time
	burstGoal      int
	burstCount     int
}

func NewDispatchWorker(
	log *log.Logger,
	repository *repository.CampaignRepository,
	settings *service.InstanceSettingsService,
	redisQueue queue.Consumer,
	provider *evolutiongo.Client,
	humanizer config.HumanizerConfig,
	retry config.RetryConfig,
) *DispatchWorker {
	return &DispatchWorker{
		log:        log,
		repository: repository,
		settings:   settings,
		redisQueue: redisQueue,
		provider:   provider,
		humanizer:  humanizer,
		retry:      retry,
		lanes:      make(map[string]*dispatchLane),
	}
}

func (w *DispatchWorker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	w.ctx = ctx
	w.cancel = cancel
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		w.run(ctx)
	}()

	w.log.Println("dispatch worker started")
}

func (w *DispatchWorker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	w.log.Println("dispatch worker stopped")
}

func (w *DispatchWorker) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		raw, err := w.redisQueue.PopCampaignMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			w.log.Printf("worker pop queue error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if raw == "" {
			continue
		}

		var job queue.CampaignMessageJob
		if err := json.Unmarshal([]byte(raw), &job); err != nil {
			w.log.Printf("worker invalid job payload: %v", err)
			continue
		}

		if err := w.handleJob(ctx, raw, job); err != nil {
			w.log.Printf("worker handle job failed: message_id=%s err=%v", job.MessageID, err)
		}
	}
}

func (w *DispatchWorker) handleJob(ctx context.Context, raw string, job queue.CampaignMessageJob) error {
	item, err := w.repository.GetMessageByID(ctx, job.MessageID)
	if err != nil {
		return err
	}

	if item.Status == "sent" || item.Status == "delivered" || item.Status == "read" {
		w.log.Printf("worker skipping already sent message: message_id=%s", job.MessageID)
		return nil
	}

	campaignItem, err := w.repository.GetByID(ctx, item.CampaignID)
	if err != nil {
		return err
	}

	if campaignItem.Paused || campaignItem.Status == "paused" {
		if err := w.redisQueue.RequeueCampaignMessage(ctx, raw); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		w.log.Printf("worker requeued paused campaign message: campaign_id=%s message_id=%s", item.CampaignID, item.ID)
		return nil
	}

	if err := w.repository.MarkMessageProcessing(ctx, job.MessageID); err != nil {
		return err
	}
	if _, err := w.repository.RecalculateCampaign(ctx, item.CampaignID); err != nil {
		w.log.Printf("worker recalculate campaign after processing failed: campaign_id=%s err=%v", item.CampaignID, err)
	}

	providerDelay, waitApplied, err := w.waitForHumanCadence(ctx, job.InstanceID)
	if err != nil {
		return err
	}
	if waitApplied > 0 {
		w.log.Printf("worker cadence wait applied: instance_id=%s message_id=%s wait=%s provider_delay_ms=%d", job.InstanceID, item.ID, waitApplied.Round(time.Second), providerDelay)
	}

	response, err := w.provider.SendText(ctx, evolutiongo.SendTextInput{
		InstanceID: job.InstanceID,
		Number:     item.RecipientPhone,
		Text:       item.Content,
		ID:         item.ID,
		Delay:      providerDelay,
	})
	if err != nil {
		attempt := item.AttemptCount + 1
		if w.shouldRetry(err, attempt) {
			nextRetryAt := time.Now().Add(w.retry.Delay)
			_ = w.repository.MarkMessagePendingRetry(ctx, job.MessageID, err.Error(), nextRetryAt)
			if _, recalcErr := w.repository.RecalculateCampaign(ctx, item.CampaignID); recalcErr != nil {
				w.log.Printf("worker recalculate campaign after retry scheduling failed: campaign_id=%s err=%v", item.CampaignID, recalcErr)
			}
			w.scheduleRetry(raw, job, nextRetryAt)
			w.log.Printf("worker scheduled retry: campaign_id=%s message_id=%s attempt=%d next_retry_at=%s", item.CampaignID, item.ID, attempt, nextRetryAt.UTC().Format(time.RFC3339))
			return err
		}

		_ = w.repository.MarkMessageFailed(ctx, job.MessageID, err.Error())
		if _, recalcErr := w.repository.RecalculateCampaign(ctx, item.CampaignID); recalcErr != nil {
			w.log.Printf("worker recalculate campaign after failure failed: campaign_id=%s err=%v", item.CampaignID, recalcErr)
		}
		return err
	}

	if err := w.repository.MarkMessageSent(ctx, job.MessageID, response.MessageID); err != nil {
		return err
	}
	if _, err := w.repository.RecalculateCampaign(ctx, item.CampaignID); err != nil {
		w.log.Printf("worker recalculate campaign after send failed: campaign_id=%s err=%v", item.CampaignID, err)
	}

	w.log.Printf("worker sent message: campaign_id=%s message_id=%s provider_message_id=%s", item.CampaignID, item.ID, response.MessageID)
	return nil
}

func (w *DispatchWorker) scheduleRetry(raw string, job queue.CampaignMessageJob, nextRetryAt time.Time) {
	job.AttemptCount++
	payload, err := json.Marshal(job)
	if err != nil {
		w.log.Printf("worker marshal retry payload failed: message_id=%s err=%v", job.MessageID, err)
		payload = []byte(raw)
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		timer := time.NewTimer(time.Until(nextRetryAt))
		defer timer.Stop()

		select {
		case <-timer.C:
			if err := w.redisQueue.RequeueCampaignMessage(context.Background(), string(payload)); err != nil {
				w.log.Printf("worker retry requeue failed: message_id=%s err=%v", job.MessageID, err)
			}
		case <-w.ctx.Done():
			return
		}
	}()
}

func (w *DispatchWorker) shouldRetry(err error, attempt int) bool {
	if attempt >= w.retry.MaxAttempts {
		return false
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "status 400"),
		strings.Contains(message, "status 401"),
		strings.Contains(message, "status 403"),
		strings.Contains(message, "status 404"),
		strings.Contains(message, "invalid json"),
		strings.Contains(message, "not authorized"):
		return false
	default:
		return true
	}
}

func (w *DispatchWorker) waitForHumanCadence(ctx context.Context, instanceID string) (int, time.Duration, error) {
	humanizerCfg := w.humanizer
	if w.settings != nil {
		if item, err := w.settings.HumanizerConfig(ctx, instanceID); err == nil {
			humanizerCfg = item
		}
	}

	if !humanizerCfg.Enabled {
		return 0, 0, nil
	}

	lane := w.dispatchLane(instanceID)
	lane.mu.Lock()
	defer lane.mu.Unlock()

	now := time.Now()
	waitFor := time.Duration(0)

	if lane.lastDispatchAt.IsZero() {
		waitFor = w.randomDuration(humanizerCfg.InitialDelayMin, humanizerCfg.InitialDelayMax)
	} else {
		baseDelay := w.randomDuration(humanizerCfg.BaseDelayMin, humanizerCfg.BaseDelayMax)
		waitUntil := lane.lastDispatchAt.Add(baseDelay)
		if until := time.Until(waitUntil); until > waitFor {
			waitFor = until
		}
	}

	if lane.burstGoal == 0 {
		lane.burstGoal = w.randomInt(humanizerCfg.BurstSizeMin, humanizerCfg.BurstSizeMax)
	}

	if lane.burstCount >= lane.burstGoal {
		burstPause := w.randomDuration(humanizerCfg.BurstPauseMin, humanizerCfg.BurstPauseMax)
		waitUntil := lane.lastDispatchAt.Add(burstPause)
		if until := time.Until(waitUntil); until > waitFor {
			waitFor = until
		}
		lane.burstCount = 0
		lane.burstGoal = w.randomInt(humanizerCfg.BurstSizeMin, humanizerCfg.BurstSizeMax)
	}

	if waitFor > 0 {
		timer := time.NewTimer(waitFor)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		case <-timer.C:
		}
	}

	lane.lastDispatchAt = now.Add(waitFor)
	lane.burstCount++

	providerDelay := w.randomDuration(humanizerCfg.ProviderDelayMin, humanizerCfg.ProviderDelayMax)
	return int(providerDelay.Milliseconds()), waitFor, nil
}

func (w *DispatchWorker) dispatchLane(instanceID string) *dispatchLane {
	w.mu.Lock()
	defer w.mu.Unlock()

	lane, ok := w.lanes[instanceID]
	if ok {
		return lane
	}

	lane = &dispatchLane{}
	w.lanes[instanceID] = lane
	return lane
}

func (w *DispatchWorker) randomDuration(min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}

	span := max - min
	return min + time.Duration(rand.Int63n(int64(span)+1))
}

func (w *DispatchWorker) randomInt(min, max int) int {
	if max <= min {
		return min
	}

	return min + rand.Intn(max-min+1)
}
