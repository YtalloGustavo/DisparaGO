package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"disparago/internal/repository"
)

var ErrUnsupportedWebhookEvent = errors.New("unsupported webhook event")

type WebhookService struct {
	repository *repository.CampaignRepository
}

type EvolutionWebhookInput struct {
	Event      string
	Status     string
	MessageIDs []string
}

type EvolutionWebhookResult struct {
	AffectedCampaigns []string `json:"affected_campaigns"`
	UpdatedMessages   int      `json:"updated_messages"`
}

func NewWebhookService(repository *repository.CampaignRepository) *WebhookService {
	return &WebhookService{repository: repository}
}

func (s *WebhookService) Track(ctx context.Context, input EvolutionWebhookInput) (EvolutionWebhookResult, error) {
	status := normalizeWebhookStatus(input.Event, input.Status)
	if status == "" {
		return EvolutionWebhookResult{}, ErrUnsupportedWebhookEvent
	}

	campaignIDs := make(map[string]struct{})
	updatedMessages := 0

	for _, messageID := range input.MessageIDs {
		messageID = strings.TrimSpace(messageID)
		if messageID == "" {
			continue
		}

		var (
			campaignID string
			err        error
		)

		switch status {
		case "delivered":
			campaignID, err = s.repository.MarkMessageDeliveredByProviderID(ctx, messageID)
		case "read":
			campaignID, err = s.repository.MarkMessageReadByProviderID(ctx, messageID)
		}
		if err != nil {
			if errors.Is(err, repository.ErrMessageNotFound) {
				continue
			}
			return EvolutionWebhookResult{}, err
		}

		updatedMessages++
		campaignIDs[campaignID] = struct{}{}
	}

	affected := make([]string, 0, len(campaignIDs))
	for campaignID := range campaignIDs {
		if _, err := s.repository.RecalculateCampaign(ctx, campaignID); err != nil {
			return EvolutionWebhookResult{}, fmt.Errorf("recalculate campaign after webhook: %w", err)
		}
		affected = append(affected, campaignID)
	}

	return EvolutionWebhookResult{
		AffectedCampaigns: affected,
		UpdatedMessages:   updatedMessages,
	}, nil
}

func normalizeWebhookStatus(event, status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "delivered":
		return "delivered"
	case "read":
		return "read"
	}

	event = strings.ToLower(strings.TrimSpace(event))
	switch {
	case strings.Contains(event, "read"):
		return "read"
	case strings.Contains(event, "deliver"):
		return "delivered"
	default:
		return ""
	}
}
