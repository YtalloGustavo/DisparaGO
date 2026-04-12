package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"disparago/internal/config"
	"disparago/internal/service"
)

type CampaignScheduler struct {
	log     *log.Logger
	service *service.CampaignService
	cfg     config.SchedulerConfig
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewCampaignScheduler(log *log.Logger, service *service.CampaignService, cfg config.SchedulerConfig) *CampaignScheduler {
	return &CampaignScheduler{
		log:     log,
		service: service,
		cfg:     cfg,
	}
}

func (s *CampaignScheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		s.run(ctx)
	}()

	s.log.Println("campaign scheduler started")
}

func (s *CampaignScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.log.Println("campaign scheduler stopped")
}

func (s *CampaignScheduler) run(ctx context.Context) {
	interval := s.cfg.PollInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := s.service.ReleaseDueScheduled(ctx, s.cfg.BatchSize); err != nil && !errors.Is(err, context.Canceled) {
			s.log.Printf("campaign scheduler tick failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
