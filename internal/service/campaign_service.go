package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	authdomain "disparago/internal/domain/auth"
	"disparago/internal/domain/campaign"
	"disparago/internal/domain/message"
	"disparago/internal/queue"
	"disparago/internal/repository"
)

var (
	ErrInvalidCampaignInput = errors.New("invalid campaign input")
	ErrCampaignNotFound     = errors.New("campaign not found")
	ErrInvalidCampaignState = errors.New("invalid campaign state")
)

type CampaignService struct {
	log        *log.Logger
	repository *repository.CampaignRepository
	publisher  *queue.Publisher
}

type CreateCampaignInput struct {
	CompanyID        int64
	CreatedByUserID  *int64
	Name             string
	InstanceID       string
	Message          string
	Contacts         []string
	SendMode         string
	ScheduledAt      string
	Timezone         string
	ExternalSource   string
	ExternalSourceID string
}

type ListCampaignsInput struct {
	CompanyID int64
	Statuses  []string
	Limit     int
}

type UpdateScheduleInput struct {
	CompanyID   int64
	CampaignID  string
	ScheduledAt string
	Timezone    string
}

type CreateCampaignResult struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	InstanceID    string          `json:"instance_id"`
	Status        campaign.Status `json:"status"`
	TotalMessages int             `json:"total_messages"`
	SendMode      string          `json:"send_mode"`
}

func NewCampaignService(log *log.Logger, repository *repository.CampaignRepository, publisher *queue.Publisher) *CampaignService {
	return &CampaignService{
		log:        log,
		repository: repository,
		publisher:  publisher,
	}
}

func (s *CampaignService) Create(ctx context.Context, input CreateCampaignInput) (CreateCampaignResult, error) {
	normalized, scheduledAtUTC, scheduledOriginal, err := normalizeCreateCampaignInput(input)
	if err != nil {
		return CreateCampaignResult{}, err
	}

	if normalized.ExternalSource != "" && normalized.ExternalSourceID != "" {
		existing, err := s.repository.FindByExternalRef(ctx, normalized.ExternalSource, normalized.ExternalSourceID)
		if err == nil {
			if existing.CompanyID != normalized.CompanyID {
				return CreateCampaignResult{}, fmt.Errorf("%w: external campaign belongs to another company", ErrInvalidCampaignInput)
			}
			if normalized.SendMode == "scheduled" {
				updated, err := s.repository.UpdateScheduledCampaign(ctx, normalized.CompanyID, existing.ID, *scheduledAtUTC, normalized.Timezone, *scheduledOriginal)
				if err != nil {
					return CreateCampaignResult{}, err
				}
				return buildCreateResult(updated), nil
			}
		} else if !errors.Is(err, repository.ErrCampaignNotFound) {
			return CreateCampaignResult{}, err
		}
	}

	campaignID := uuid.New()
	messageParams := make([]repository.CreateMessageParams, 0, len(normalized.Contacts))
	jobs := make([]queue.CampaignMessageJob, 0, len(normalized.Contacts))
	for _, contact := range normalized.Contacts {
		messageID := uuid.New()
		messageParams = append(messageParams, repository.CreateMessageParams{
			ID:             messageID,
			RecipientPhone: contact,
			Content:        normalized.Message,
			Status:         message.StatusPending,
		})
		jobs = append(jobs, queue.CampaignMessageJob{
			MessageID:    messageID.String(),
			CampaignID:   campaignID.String(),
			InstanceID:   normalized.InstanceID,
			Recipient:    contact,
			Message:      normalized.Message,
			AttemptCount: 0,
		})
	}

	status := campaign.StatusPending
	if normalized.SendMode == "scheduled" {
		status = campaign.StatusScheduled
	}

	createdCampaign, _, err := s.repository.Create(ctx, repository.CreateCampaignParams{
		ID:               campaignID,
		CompanyID:        normalized.CompanyID,
		CreatedByUserID:  normalized.CreatedByUserID,
		Name:             normalized.Name,
		InstanceID:       normalized.InstanceID,
		Message:          normalized.Message,
		Status:           status,
		SendMode:         normalized.SendMode,
		ScheduledAtUTC:   scheduledAtUTC,
		ScheduledTZ:      normalized.Timezone,
		ScheduledAt:      scheduledOriginal,
		ExternalSource:   normalized.ExternalSource,
		ExternalSourceID: normalized.ExternalSourceID,
		Messages:         messageParams,
	})
	if err != nil {
		return CreateCampaignResult{}, fmt.Errorf("create campaign: %w", err)
	}

	if normalized.SendMode == "now" {
		if err := s.publisher.PublishCampaignMessages(ctx, jobs); err != nil {
			return CreateCampaignResult{}, fmt.Errorf("publish campaign jobs: %w", err)
		}
	}

	s.log.Printf(
		"campaign created: id=%s company_id=%d instance_id=%s contacts=%d mode=%s",
		createdCampaign.ID,
		createdCampaign.CompanyID,
		createdCampaign.InstanceID,
		createdCampaign.TotalCount,
		createdCampaign.SendMode,
	)

	return buildCreateResult(createdCampaign), nil
}

func (s *CampaignService) Get(ctx context.Context, actor authdomain.Actor, campaignID string) (campaign.Campaign, error) {
	item, err := s.repository.GetByID(ctx, actor.CompanyID, campaignID, actor.IsSuperadmin())
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, err
	}
	return item, nil
}

func (s *CampaignService) List(ctx context.Context, actor authdomain.Actor, input ListCampaignsInput) ([]campaign.Campaign, error) {
	if !actor.IsSuperadmin() {
		input.CompanyID = actor.CompanyID
	}

	items, err := s.repository.ListCampaigns(ctx, repository.ListCampaignsFilter{
		CompanyID: input.CompanyID,
		Statuses:  input.Statuses,
		Limit:     input.Limit,
	}, actor.IsSuperadmin())
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *CampaignService) ListMessages(ctx context.Context, actor authdomain.Actor, campaignID string) ([]message.Message, error) {
	items, err := s.repository.ListMessagesByCampaignID(ctx, actor.CompanyID, campaignID, actor.IsSuperadmin())
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return nil, ErrCampaignNotFound
		}
		return nil, err
	}
	return items, nil
}

func (s *CampaignService) Pause(ctx context.Context, actor authdomain.Actor, campaignID string) (campaign.Campaign, error) {
	item, err := s.Get(ctx, actor, campaignID)
	if err != nil {
		return campaign.Campaign{}, err
	}

	switch item.Status {
	case campaign.StatusSent, campaign.StatusDelivered, campaign.StatusRead, campaign.StatusFailed, campaign.StatusPartial:
		return campaign.Campaign{}, fmt.Errorf("%w: finalized campaigns cannot be paused", ErrInvalidCampaignState)
	case campaign.StatusScheduled:
		return campaign.Campaign{}, fmt.Errorf("%w: scheduled campaigns must be canceled or rescheduled", ErrInvalidCampaignState)
	}

	paused, err := s.repository.SetCampaignPaused(ctx, campaignID, true)
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, err
	}

	return paused, nil
}

func (s *CampaignService) Resume(ctx context.Context, actor authdomain.Actor, campaignID string) (campaign.Campaign, error) {
	item, err := s.Get(ctx, actor, campaignID)
	if err != nil {
		return campaign.Campaign{}, err
	}
	if !item.Paused {
		return campaign.Campaign{}, fmt.Errorf("%w: campaign is not paused", ErrInvalidCampaignState)
	}

	if _, err := s.repository.SetCampaignPaused(ctx, campaignID, false); err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, err
	}

	resumed, err := s.repository.RecalculateCampaign(ctx, campaignID)
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, err
	}
	return resumed, nil
}

func (s *CampaignService) Reschedule(ctx context.Context, actor authdomain.Actor, input UpdateScheduleInput) (campaign.Campaign, error) {
	item, err := s.Get(ctx, actor, input.CampaignID)
	if err != nil {
		return campaign.Campaign{}, err
	}
	if item.ReleasedAt != nil {
		return campaign.Campaign{}, fmt.Errorf("%w: released campaigns cannot be rescheduled", ErrInvalidCampaignState)
	}

	scheduledAtUTC, scheduledOriginal, location, err := normalizeScheduleInput(input.ScheduledAt, input.Timezone)
	if err != nil {
		return campaign.Campaign{}, err
	}

	updated, err := s.repository.UpdateScheduledCampaign(ctx, item.CompanyID, item.ID, *scheduledAtUTC, location.String(), *scheduledOriginal)
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, err
	}
	return updated, nil
}

func (s *CampaignService) CancelScheduled(ctx context.Context, actor authdomain.Actor, campaignID string) (campaign.Campaign, error) {
	item, err := s.Get(ctx, actor, campaignID)
	if err != nil {
		return campaign.Campaign{}, err
	}
	if item.ReleasedAt != nil {
		return campaign.Campaign{}, fmt.Errorf("%w: released campaigns cannot be canceled", ErrInvalidCampaignState)
	}
	if item.Status != campaign.StatusScheduled {
		return campaign.Campaign{}, fmt.Errorf("%w: only scheduled campaigns can be canceled", ErrInvalidCampaignState)
	}

	canceled, err := s.repository.CancelScheduledCampaign(ctx, item.CompanyID, item.ID)
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, err
	}
	return canceled, nil
}

func (s *CampaignService) ReleaseDueScheduled(ctx context.Context, batchSize int) error {
	claimed, err := s.repository.ClaimDueScheduledCampaigns(ctx, batchSize)
	if err != nil {
		return err
	}

	for _, item := range claimed {
		jobs := make([]queue.CampaignMessageJob, 0, len(item.Messages))
		for _, msg := range item.Messages {
			jobs = append(jobs, queue.CampaignMessageJob{
				MessageID:    msg.ID,
				CampaignID:   item.Campaign.ID,
				InstanceID:   item.Campaign.InstanceID,
				Recipient:    msg.RecipientPhone,
				Message:      msg.Content,
				AttemptCount: msg.AttemptCount,
			})
		}

		if err := s.publisher.PublishCampaignMessages(ctx, jobs); err != nil {
			_ = s.repository.RevertScheduledCampaignRelease(ctx, item.Campaign.ID)
			return fmt.Errorf("publish scheduled campaign jobs: %w", err)
		}

		s.log.Printf("scheduled campaign released: id=%s company_id=%d messages=%d", item.Campaign.ID, item.Campaign.CompanyID, len(item.Messages))
	}

	return nil
}

func normalizeCreateCampaignInput(input CreateCampaignInput) (CreateCampaignInput, *time.Time, *time.Time, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.InstanceID = strings.TrimSpace(input.InstanceID)
	input.Message = strings.TrimSpace(input.Message)
	input.SendMode = strings.ToLower(strings.TrimSpace(input.SendMode))
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.ExternalSource = strings.TrimSpace(input.ExternalSource)
	input.ExternalSourceID = strings.TrimSpace(input.ExternalSourceID)

	if input.CompanyID <= 0 || input.Name == "" || input.InstanceID == "" || input.Message == "" {
		return CreateCampaignInput{}, nil, nil, fmt.Errorf("%w: company_id, name, instance_id and message are required", ErrInvalidCampaignInput)
	}
	if input.SendMode == "" {
		input.SendMode = "now"
	}
	if input.SendMode != "now" && input.SendMode != "scheduled" {
		return CreateCampaignInput{}, nil, nil, fmt.Errorf("%w: send_mode must be now or scheduled", ErrInvalidCampaignInput)
	}

	contacts := make([]string, 0, len(input.Contacts))
	for _, contact := range input.Contacts {
		trimmed := strings.TrimSpace(contact)
		if trimmed != "" {
			contacts = append(contacts, trimmed)
		}
	}
	if len(contacts) == 0 {
		return CreateCampaignInput{}, nil, nil, fmt.Errorf("%w: at least one contact is required", ErrInvalidCampaignInput)
	}
	input.Contacts = contacts

	if input.SendMode == "scheduled" {
		scheduledAtUTC, scheduledOriginal, location, err := normalizeScheduleInput(input.ScheduledAt, input.Timezone)
		if err != nil {
			return CreateCampaignInput{}, nil, nil, err
		}
		input.Timezone = location.String()
		return input, scheduledAtUTC, scheduledOriginal, nil
	}

	return input, nil, nil, nil
}

func normalizeScheduleInput(rawValue, rawTimezone string) (*time.Time, *time.Time, *time.Location, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return nil, nil, nil, fmt.Errorf("%w: scheduled_at is required for scheduled campaigns", ErrInvalidCampaignInput)
	}
	timezone := strings.TrimSpace(rawTimezone)
	if timezone == "" {
		timezone = "America/Sao_Paulo"
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: invalid timezone", ErrInvalidCampaignInput)
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}

	var parsed time.Time
	for _, layout := range layouts {
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, location)
		}
		if err == nil {
			utc := parsed.UTC()
			local := parsed.In(location)
			return &utc, &local, location, nil
		}
	}

	return nil, nil, nil, fmt.Errorf("%w: invalid scheduled_at", ErrInvalidCampaignInput)
}

func buildCreateResult(item campaign.Campaign) CreateCampaignResult {
	return CreateCampaignResult{
		ID:            item.ID,
		Name:          item.Name,
		InstanceID:    item.InstanceID,
		Status:        item.Status,
		TotalMessages: item.TotalCount,
		SendMode:      item.SendMode,
	}
}
