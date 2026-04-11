package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

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
	Name       string
	InstanceID string
	Message    string
	Contacts   []string
}

type CreateCampaignResult struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	InstanceID    string          `json:"instance_id"`
	Status        campaign.Status `json:"status"`
	TotalMessages int             `json:"total_messages"`
}

func NewCampaignService(log *log.Logger, repository *repository.CampaignRepository, publisher *queue.Publisher) *CampaignService {
	return &CampaignService{
		log:        log,
		repository: repository,
		publisher:  publisher,
	}
}

func (s *CampaignService) Create(ctx context.Context, input CreateCampaignInput) (CreateCampaignResult, error) {
	normalized, err := normalizeCreateCampaignInput(input)
	if err != nil {
		return CreateCampaignResult{}, err
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

	createdCampaign, _, err := s.repository.Create(ctx, repository.CreateCampaignParams{
		ID:         campaignID,
		Name:       normalized.Name,
		InstanceID: normalized.InstanceID,
		Message:    normalized.Message,
		Status:     campaign.StatusPending,
		Messages:   messageParams,
	})
	if err != nil {
		return CreateCampaignResult{}, fmt.Errorf("create campaign: %w", err)
	}

	if err := s.publisher.PublishCampaignMessages(ctx, jobs); err != nil {
		return CreateCampaignResult{}, fmt.Errorf("publish campaign jobs: %w", err)
	}

	s.log.Printf("campaign created: id=%s instance_id=%s contacts=%d", createdCampaign.ID, createdCampaign.InstanceID, createdCampaign.TotalCount)

	return CreateCampaignResult{
		ID:            createdCampaign.ID,
		Name:          createdCampaign.Name,
		InstanceID:    createdCampaign.InstanceID,
		Status:        createdCampaign.Status,
		TotalMessages: createdCampaign.TotalCount,
	}, nil
}

func (s *CampaignService) Get(ctx context.Context, campaignID string) (campaign.Campaign, error) {
	item, err := s.repository.GetByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, err
	}

	return item, nil
}

func (s *CampaignService) List(ctx context.Context, limit int) ([]campaign.Campaign, error) {
	items, err := s.repository.ListCampaigns(ctx, limit)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (s *CampaignService) ListMessages(ctx context.Context, campaignID string) ([]message.Message, error) {
	items, err := s.repository.ListMessagesByCampaignID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return nil, ErrCampaignNotFound
		}
		return nil, err
	}

	return items, nil
}

func (s *CampaignService) Pause(ctx context.Context, campaignID string) (campaign.Campaign, error) {
	item, err := s.repository.GetByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, err
	}

	switch item.Status {
	case campaign.StatusSent, campaign.StatusDelivered, campaign.StatusRead, campaign.StatusFailed, campaign.StatusPartial:
		return campaign.Campaign{}, fmt.Errorf("%w: finalized campaigns cannot be paused", ErrInvalidCampaignState)
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

func (s *CampaignService) Resume(ctx context.Context, campaignID string) (campaign.Campaign, error) {
	item, err := s.repository.GetByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, repository.ErrCampaignNotFound) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
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

func normalizeCreateCampaignInput(input CreateCampaignInput) (CreateCampaignInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.InstanceID = strings.TrimSpace(input.InstanceID)
	input.Message = strings.TrimSpace(input.Message)

	if input.Name == "" || input.InstanceID == "" || input.Message == "" {
		return CreateCampaignInput{}, fmt.Errorf("%w: name, instance_id and message are required", ErrInvalidCampaignInput)
	}

	contacts := make([]string, 0, len(input.Contacts))
	for _, contact := range input.Contacts {
		trimmed := strings.TrimSpace(contact)
		if trimmed == "" {
			continue
		}
		contacts = append(contacts, trimmed)
	}

	if len(contacts) == 0 {
		return CreateCampaignInput{}, fmt.Errorf("%w: at least one contact is required", ErrInvalidCampaignInput)
	}

	input.Contacts = contacts
	return input, nil
}
