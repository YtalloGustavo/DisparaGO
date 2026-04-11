package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"disparago/internal/domain/campaign"
	"disparago/internal/domain/message"
	"disparago/internal/platform/database"
)

var ErrCampaignNotFound = errors.New("campaign not found")
var ErrMessageNotFound = errors.New("message not found")

type CampaignRepository struct {
	db *database.Client
}

type CreateCampaignParams struct {
	ID         uuid.UUID
	Name       string
	InstanceID string
	Message    string
	Status     campaign.Status
	Messages   []CreateMessageParams
}

type CreateMessageParams struct {
	ID             uuid.UUID
	RecipientPhone string
	Content        string
	Status         message.Status
}

func NewCampaignRepository(db *database.Client) *CampaignRepository {
	return &CampaignRepository{db: db}
}

func (r *CampaignRepository) Create(ctx context.Context, params CreateCampaignParams) (campaign.Campaign, []message.Message, error) {
	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return campaign.Campaign{}, nil, fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		INSERT INTO campaigns (
			id,
			name,
			instance_id,
			message_content,
			status,
			total_messages,
			pending_count,
			processing_count,
			sent_count,
			delivered_count,
			read_count,
			failed_count,
			paused,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, params.ID, params.Name, params.InstanceID, params.Message, params.Status, len(params.Messages), len(params.Messages), 0, 0, 0, 0, 0, false, now, now)
	if err != nil {
		return campaign.Campaign{}, nil, fmt.Errorf("insert campaign: %w", err)
	}

	createdMessages := make([]message.Message, 0, len(params.Messages))
	for _, item := range params.Messages {
		_, err = tx.Exec(ctx, `
			INSERT INTO campaign_messages (
				id,
				campaign_id,
				recipient_phone,
				message_content,
				status,
				attempt_count,
				created_at,
				updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, item.ID, params.ID, item.RecipientPhone, item.Content, item.Status, 0, now, now)
		if err != nil {
			return campaign.Campaign{}, nil, fmt.Errorf("insert campaign message: %w", err)
		}

		createdMessages = append(createdMessages, message.Message{
			ID:                item.ID.String(),
			CampaignID:        params.ID.String(),
			RecipientPhone:    item.RecipientPhone,
			Content:           item.Content,
			Status:            item.Status,
			ProviderMessageID: "",
			AttemptCount:      0,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return campaign.Campaign{}, nil, fmt.Errorf("commit transaction: %w", err)
	}

	return campaign.Campaign{
		ID:              params.ID.String(),
		Name:            params.Name,
		InstanceID:      params.InstanceID,
		Message:         params.Message,
		Status:          params.Status,
		TotalCount:      len(params.Messages),
		PendingCount:    len(params.Messages),
		ProcessingCount: 0,
		SentCount:       0,
		DeliveredCount:  0,
		ReadCount:       0,
		FailedCount:     0,
		Paused:          false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, createdMessages, nil
}

func (r *CampaignRepository) GetByID(ctx context.Context, campaignID string) (campaign.Campaign, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, instance_id, message_content, status, total_messages,
		       pending_count, processing_count, sent_count, delivered_count, read_count, failed_count, paused, created_at, updated_at
		FROM campaigns
		WHERE id = $1
	`, campaignID)

	var item campaign.Campaign
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.InstanceID,
		&item.Message,
		&item.Status,
		&item.TotalCount,
		&item.PendingCount,
		&item.ProcessingCount,
		&item.SentCount,
		&item.DeliveredCount,
		&item.ReadCount,
		&item.FailedCount,
		&item.Paused,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}

		return campaign.Campaign{}, fmt.Errorf("query campaign: %w", err)
	}

	return item, nil
}

func (r *CampaignRepository) ListCampaigns(ctx context.Context, limit int) ([]campaign.Campaign, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, name, instance_id, message_content, status, total_messages,
		       pending_count, processing_count, sent_count, delivered_count, read_count, failed_count, paused, created_at, updated_at
		FROM campaigns
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()

	items := make([]campaign.Campaign, 0, limit)
	for rows.Next() {
		var item campaign.Campaign
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.InstanceID,
			&item.Message,
			&item.Status,
			&item.TotalCount,
			&item.PendingCount,
			&item.ProcessingCount,
			&item.SentCount,
			&item.DeliveredCount,
			&item.ReadCount,
			&item.FailedCount,
			&item.Paused,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate campaigns: %w", err)
	}

	return items, nil
}

func (r *CampaignRepository) GetMessageByID(ctx context.Context, messageID string) (message.Message, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, campaign_id, recipient_phone, message_content, status, COALESCE(provider_message_id, ''), attempt_count,
		       COALESCE(last_error, ''), next_retry_at, sent_at, delivered_at, read_at, failed_at, created_at, updated_at
		FROM campaign_messages
		WHERE id = $1
	`, messageID)

	var item message.Message
	if err := row.Scan(
		&item.ID,
		&item.CampaignID,
		&item.RecipientPhone,
		&item.Content,
		&item.Status,
		&item.ProviderMessageID,
		&item.AttemptCount,
		&item.LastError,
		&item.NextRetryAt,
		&item.SentAt,
		&item.DeliveredAt,
		&item.ReadAt,
		&item.FailedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return message.Message{}, ErrMessageNotFound
		}

		return message.Message{}, fmt.Errorf("query campaign message: %w", err)
	}

	return item, nil
}

func (r *CampaignRepository) ListMessagesByCampaignID(ctx context.Context, campaignID string) ([]message.Message, error) {
	if _, err := r.GetByID(ctx, campaignID); err != nil {
		return nil, err
	}

	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, campaign_id, recipient_phone, message_content, status, COALESCE(provider_message_id, ''), attempt_count,
		       COALESCE(last_error, ''), next_retry_at, sent_at, delivered_at, read_at, failed_at, created_at, updated_at
		FROM campaign_messages
		WHERE campaign_id = $1
		ORDER BY created_at ASC
	`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list campaign messages: %w", err)
	}
	defer rows.Close()

	items := make([]message.Message, 0)
	for rows.Next() {
		var item message.Message
		if err := rows.Scan(
			&item.ID,
			&item.CampaignID,
			&item.RecipientPhone,
			&item.Content,
			&item.Status,
			&item.ProviderMessageID,
			&item.AttemptCount,
			&item.LastError,
			&item.NextRetryAt,
			&item.SentAt,
			&item.DeliveredAt,
			&item.ReadAt,
			&item.FailedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan campaign message: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate campaign messages: %w", err)
	}

	return items, nil
}

func (r *CampaignRepository) MarkMessageProcessing(ctx context.Context, messageID string) error {
	commandTag, err := r.db.Pool.Exec(ctx, `
		UPDATE campaign_messages
		SET status = 'processing',
		    attempt_count = attempt_count + 1,
		    last_error = NULL,
		    next_retry_at = NULL,
		    failed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`, messageID)
	if err != nil {
		return fmt.Errorf("mark message processing: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrMessageNotFound
	}

	return nil
}

func (r *CampaignRepository) RecalculateCampaign(ctx context.Context, campaignID string) (campaign.Campaign, error) {
	row := r.db.Pool.QueryRow(ctx, `
		WITH counts AS (
			SELECT
				c.id,
				c.total_messages,
				COUNT(cm.id) FILTER (WHERE cm.status = 'pending') AS pending_count,
				COUNT(cm.id) FILTER (WHERE cm.status = 'processing') AS processing_count,
				COUNT(cm.id) FILTER (WHERE cm.status = 'sent') AS sent_count,
				COUNT(cm.id) FILTER (WHERE cm.status = 'delivered') AS delivered_count,
				COUNT(cm.id) FILTER (WHERE cm.status = 'read') AS read_count,
				COUNT(cm.id) FILTER (WHERE cm.status = 'failed') AS failed_count
			FROM campaigns c
			LEFT JOIN campaign_messages cm ON cm.campaign_id = c.id
			WHERE c.id = $1
			GROUP BY c.id, c.total_messages
		)
		UPDATE campaigns c
		SET pending_count = counts.pending_count,
		    processing_count = counts.processing_count,
		    sent_count = counts.sent_count,
		    delivered_count = counts.delivered_count,
		    read_count = counts.read_count,
		    failed_count = counts.failed_count,
		    status = CASE
				WHEN c.paused THEN 'paused'
				WHEN counts.total_messages = 0 THEN 'draft'
				WHEN counts.read_count = counts.total_messages THEN 'read'
				WHEN counts.delivered_count + counts.read_count = counts.total_messages THEN 'delivered'
				WHEN counts.sent_count + counts.delivered_count + counts.read_count = counts.total_messages THEN 'sent'
				WHEN counts.processing_count > 0 THEN 'processing'
				WHEN counts.pending_count = counts.total_messages THEN 'pending'
				WHEN counts.failed_count = counts.total_messages THEN 'failed'
				WHEN counts.sent_count + counts.delivered_count + counts.read_count > 0 AND counts.failed_count > 0 AND counts.pending_count = 0 AND counts.processing_count = 0 THEN 'partial'
				WHEN counts.pending_count > 0 THEN 'processing'
				ELSE c.status
			END,
		    updated_at = NOW()
		FROM counts
		WHERE c.id = counts.id
		RETURNING c.id, c.name, c.instance_id, c.message_content, c.status, c.total_messages,
		          c.pending_count, c.processing_count, c.sent_count, c.delivered_count, c.read_count, c.failed_count, c.paused, c.created_at, c.updated_at
	`, campaignID)

	var item campaign.Campaign
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.InstanceID,
		&item.Message,
		&item.Status,
		&item.TotalCount,
		&item.PendingCount,
		&item.ProcessingCount,
		&item.SentCount,
		&item.DeliveredCount,
		&item.ReadCount,
		&item.FailedCount,
		&item.Paused,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, fmt.Errorf("recalculate campaign: %w", err)
	}

	return item, nil
}

func (r *CampaignRepository) SetCampaignPaused(ctx context.Context, campaignID string, paused bool) (campaign.Campaign, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE campaigns
		SET paused = $2,
		    status = CASE WHEN $2 THEN 'paused' ELSE status END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, instance_id, message_content, status, total_messages,
		          pending_count, processing_count, sent_count, delivered_count, read_count, failed_count, paused, created_at, updated_at
	`, campaignID, paused)

	var item campaign.Campaign
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.InstanceID,
		&item.Message,
		&item.Status,
		&item.TotalCount,
		&item.PendingCount,
		&item.ProcessingCount,
		&item.SentCount,
		&item.DeliveredCount,
		&item.ReadCount,
		&item.FailedCount,
		&item.Paused,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return campaign.Campaign{}, ErrCampaignNotFound
		}
		return campaign.Campaign{}, fmt.Errorf("set campaign paused: %w", err)
	}

	return item, nil
}

func (r *CampaignRepository) MarkMessageDeliveredByProviderID(ctx context.Context, providerMessageID string) (string, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE campaign_messages
		SET status = CASE WHEN status = 'read' THEN 'read' ELSE 'delivered' END,
		    delivered_at = COALESCE(delivered_at, NOW()),
		    updated_at = NOW()
		WHERE provider_message_id = $1
		RETURNING campaign_id
	`, providerMessageID)

	var campaignID string
	if err := row.Scan(&campaignID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrMessageNotFound
		}
		return "", fmt.Errorf("mark message delivered: %w", err)
	}

	return campaignID, nil
}

func (r *CampaignRepository) MarkMessageReadByProviderID(ctx context.Context, providerMessageID string) (string, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE campaign_messages
		SET status = 'read',
		    delivered_at = COALESCE(delivered_at, NOW()),
		    read_at = NOW(),
		    updated_at = NOW()
		WHERE provider_message_id = $1
		RETURNING campaign_id
	`, providerMessageID)

	var campaignID string
	if err := row.Scan(&campaignID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrMessageNotFound
		}
		return "", fmt.Errorf("mark message read: %w", err)
	}

	return campaignID, nil
}

func (r *CampaignRepository) MarkMessageSent(ctx context.Context, messageID string, providerMessageID string) error {
	commandTag, err := r.db.Pool.Exec(ctx, `
		UPDATE campaign_messages
		SET status = 'sent',
		    provider_message_id = $2,
		    last_error = NULL,
		    next_retry_at = NULL,
		    sent_at = NOW(),
		    failed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`, messageID, providerMessageID)
	if err != nil {
		return fmt.Errorf("mark message sent: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrMessageNotFound
	}

	return nil
}

func (r *CampaignRepository) MarkMessageFailed(ctx context.Context, messageID string, failure string) error {
	commandTag, err := r.db.Pool.Exec(ctx, `
		UPDATE campaign_messages
		SET status = 'failed',
		    last_error = $2,
		    next_retry_at = NULL,
		    failed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, messageID, failure)
	if err != nil {
		return fmt.Errorf("mark message failed: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrMessageNotFound
	}

	return nil
}

func (r *CampaignRepository) MarkMessagePendingRetry(ctx context.Context, messageID string, failure string, nextRetryAt time.Time) error {
	commandTag, err := r.db.Pool.Exec(ctx, `
		UPDATE campaign_messages
		SET status = 'pending',
		    last_error = $2,
		    next_retry_at = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, messageID, failure, nextRetryAt.UTC())
	if err != nil {
		return fmt.Errorf("mark message pending retry: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrMessageNotFound
	}

	return nil
}
