package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	instancecfg "disparago/internal/domain/instance"
	"disparago/internal/platform/database"
)

var ErrInstanceSettingsNotFound = errors.New("instance settings not found")

type InstanceSettingsRepository struct {
	db *database.Client
}

func NewInstanceSettingsRepository(db *database.Client) *InstanceSettingsRepository {
	return &InstanceSettingsRepository{db: db}
}

func (r *InstanceSettingsRepository) Get(ctx context.Context, companyID int64, instanceID string) (instancecfg.Settings, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT company_id, instance_id, humanizer_enabled, initial_delay_min_seconds, initial_delay_max_seconds,
		       base_delay_min_seconds, base_delay_max_seconds, provider_delay_min_ms, provider_delay_max_ms,
		       burst_size_min, burst_size_max, burst_pause_min_seconds, burst_pause_max_seconds,
		       webhook_enabled, webhook_subscriptions, created_at, updated_at
		FROM instance_settings
		WHERE company_id = $1 AND instance_id = $2
	`, companyID, instanceID)

	var item instancecfg.Settings
	if err := row.Scan(
		&item.CompanyID,
		&item.InstanceID,
		&item.HumanizerEnabled,
		&item.InitialDelayMinSec,
		&item.InitialDelayMaxSec,
		&item.BaseDelayMinSec,
		&item.BaseDelayMaxSec,
		&item.ProviderDelayMinMS,
		&item.ProviderDelayMaxMS,
		&item.BurstSizeMin,
		&item.BurstSizeMax,
		&item.BurstPauseMinSec,
		&item.BurstPauseMaxSec,
		&item.WebhookEnabled,
		&item.WebhookSubscriptions,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return instancecfg.Settings{}, ErrInstanceSettingsNotFound
		}
		return instancecfg.Settings{}, fmt.Errorf("query instance settings: %w", err)
	}

	return item, nil
}

func (r *InstanceSettingsRepository) Upsert(ctx context.Context, item instancecfg.Settings) (instancecfg.Settings, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO instance_settings (
			company_id, instance_id, humanizer_enabled, initial_delay_min_seconds, initial_delay_max_seconds,
			base_delay_min_seconds, base_delay_max_seconds, provider_delay_min_ms, provider_delay_max_ms,
			burst_size_min, burst_size_max, burst_pause_min_seconds, burst_pause_max_seconds,
			webhook_enabled, webhook_subscriptions, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, NOW(), NOW()
		)
		ON CONFLICT (company_id, instance_id) DO UPDATE
		SET humanizer_enabled = EXCLUDED.humanizer_enabled,
		    initial_delay_min_seconds = EXCLUDED.initial_delay_min_seconds,
		    initial_delay_max_seconds = EXCLUDED.initial_delay_max_seconds,
		    base_delay_min_seconds = EXCLUDED.base_delay_min_seconds,
		    base_delay_max_seconds = EXCLUDED.base_delay_max_seconds,
		    provider_delay_min_ms = EXCLUDED.provider_delay_min_ms,
		    provider_delay_max_ms = EXCLUDED.provider_delay_max_ms,
		    burst_size_min = EXCLUDED.burst_size_min,
		    burst_size_max = EXCLUDED.burst_size_max,
		    burst_pause_min_seconds = EXCLUDED.burst_pause_min_seconds,
		    burst_pause_max_seconds = EXCLUDED.burst_pause_max_seconds,
		    webhook_enabled = EXCLUDED.webhook_enabled,
		    webhook_subscriptions = EXCLUDED.webhook_subscriptions,
		    updated_at = NOW()
		RETURNING company_id, instance_id, humanizer_enabled, initial_delay_min_seconds, initial_delay_max_seconds,
		          base_delay_min_seconds, base_delay_max_seconds, provider_delay_min_ms, provider_delay_max_ms,
		          burst_size_min, burst_size_max, burst_pause_min_seconds, burst_pause_max_seconds,
		          webhook_enabled, webhook_subscriptions, created_at, updated_at
	`,
		item.CompanyID,
		item.InstanceID,
		item.HumanizerEnabled,
		item.InitialDelayMinSec,
		item.InitialDelayMaxSec,
		item.BaseDelayMinSec,
		item.BaseDelayMaxSec,
		item.ProviderDelayMinMS,
		item.ProviderDelayMaxMS,
		item.BurstSizeMin,
		item.BurstSizeMax,
		item.BurstPauseMinSec,
		item.BurstPauseMaxSec,
		item.WebhookEnabled,
		item.WebhookSubscriptions,
	)

	var saved instancecfg.Settings
	if err := row.Scan(
		&saved.CompanyID,
		&saved.InstanceID,
		&saved.HumanizerEnabled,
		&saved.InitialDelayMinSec,
		&saved.InitialDelayMaxSec,
		&saved.BaseDelayMinSec,
		&saved.BaseDelayMaxSec,
		&saved.ProviderDelayMinMS,
		&saved.ProviderDelayMaxMS,
		&saved.BurstSizeMin,
		&saved.BurstSizeMax,
		&saved.BurstPauseMinSec,
		&saved.BurstPauseMaxSec,
		&saved.WebhookEnabled,
		&saved.WebhookSubscriptions,
		&saved.CreatedAt,
		&saved.UpdatedAt,
	); err != nil {
		return instancecfg.Settings{}, fmt.Errorf("upsert instance settings: %w", err)
	}
	return saved, nil
}

func (r *InstanceSettingsRepository) ListByCompany(ctx context.Context, companyID int64) ([]instancecfg.Settings, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT company_id, instance_id, humanizer_enabled, initial_delay_min_seconds, initial_delay_max_seconds,
		       base_delay_min_seconds, base_delay_max_seconds, provider_delay_min_ms, provider_delay_max_ms,
		       burst_size_min, burst_size_max, burst_pause_min_seconds, burst_pause_max_seconds,
		       webhook_enabled, webhook_subscriptions, created_at, updated_at
		FROM instance_settings
		WHERE company_id = $1
		ORDER BY instance_id ASC
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list instance settings: %w", err)
	}
	defer rows.Close()

	items := make([]instancecfg.Settings, 0)
	for rows.Next() {
		var item instancecfg.Settings
		if err := rows.Scan(
			&item.CompanyID,
			&item.InstanceID,
			&item.HumanizerEnabled,
			&item.InitialDelayMinSec,
			&item.InitialDelayMaxSec,
			&item.BaseDelayMinSec,
			&item.BaseDelayMaxSec,
			&item.ProviderDelayMinMS,
			&item.ProviderDelayMaxMS,
			&item.BurstSizeMin,
			&item.BurstSizeMax,
			&item.BurstPauseMinSec,
			&item.BurstPauseMaxSec,
			&item.WebhookEnabled,
			&item.WebhookSubscriptions,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan instance settings: %w", err)
		}
		items = append(items, item)
	}

	return items, rows.Err()
}
