package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"disparago/internal/config"
	instancecfg "disparago/internal/domain/instance"
	"disparago/internal/repository"
)

var ErrInvalidInstanceSettings = errors.New("invalid instance settings")

type InstanceSettingsService struct {
	repository *repository.InstanceSettingsRepository
	app        config.AppConfig
	humanizer  config.HumanizerConfig
	webhook    config.WebhookConfig
}

func NewInstanceSettingsService(
	repository *repository.InstanceSettingsRepository,
	app config.AppConfig,
	humanizer config.HumanizerConfig,
	webhook config.WebhookConfig,
) *InstanceSettingsService {
	return &InstanceSettingsService{
		repository: repository,
		app:        app,
		humanizer:  humanizer,
		webhook:    webhook,
	}
}

func (s *InstanceSettingsService) Get(ctx context.Context, companyID int64, instanceID string) (instancecfg.Settings, error) {
	instanceID = strings.TrimSpace(instanceID)
	if companyID <= 0 || instanceID == "" {
		return instancecfg.Settings{}, fmt.Errorf("%w: company_id and instance_id are required", ErrInvalidInstanceSettings)
	}

	item, err := s.repository.Get(ctx, companyID, instanceID)
	if err != nil {
		if !errors.Is(err, repository.ErrInstanceSettingsNotFound) {
			return instancecfg.Settings{}, err
		}
		item = s.defaultSettings(companyID, instanceID)
	}

	return s.enrich(item), nil
}

func (s *InstanceSettingsService) ListByCompany(ctx context.Context, companyID int64) ([]instancecfg.Settings, error) {
	items, err := s.repository.ListByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}

	enriched := make([]instancecfg.Settings, 0, len(items))
	for _, item := range items {
		enriched = append(enriched, s.enrich(item))
	}
	return enriched, nil
}

func (s *InstanceSettingsService) Save(ctx context.Context, item instancecfg.Settings) (instancecfg.Settings, error) {
	item.InstanceID = strings.TrimSpace(item.InstanceID)
	if item.CompanyID <= 0 || item.InstanceID == "" {
		return instancecfg.Settings{}, fmt.Errorf("%w: company_id and instance_id are required", ErrInvalidInstanceSettings)
	}
	if err := s.validate(&item); err != nil {
		return instancecfg.Settings{}, err
	}

	saved, err := s.repository.Upsert(ctx, item)
	if err != nil {
		return instancecfg.Settings{}, err
	}
	return s.enrich(saved), nil
}

func (s *InstanceSettingsService) HumanizerConfig(ctx context.Context, companyID int64, instanceID string) (config.HumanizerConfig, error) {
	item, err := s.Get(ctx, companyID, instanceID)
	if err != nil {
		return config.HumanizerConfig{}, err
	}

	return config.HumanizerConfig{
		Enabled:          item.HumanizerEnabled,
		InitialDelayMin:  time.Duration(item.InitialDelayMinSec) * time.Second,
		InitialDelayMax:  time.Duration(item.InitialDelayMaxSec) * time.Second,
		BaseDelayMin:     time.Duration(item.BaseDelayMinSec) * time.Second,
		BaseDelayMax:     time.Duration(item.BaseDelayMaxSec) * time.Second,
		ProviderDelayMin: time.Duration(item.ProviderDelayMinMS) * time.Millisecond,
		ProviderDelayMax: time.Duration(item.ProviderDelayMaxMS) * time.Millisecond,
		BurstSizeMin:     item.BurstSizeMin,
		BurstSizeMax:     item.BurstSizeMax,
		BurstPauseMin:    time.Duration(item.BurstPauseMinSec) * time.Second,
		BurstPauseMax:    time.Duration(item.BurstPauseMaxSec) * time.Second,
	}, nil
}

func (s *InstanceSettingsService) ValidateWebhookToken(ctx context.Context, companyID int64, instanceID, token string) error {
	settings, err := s.Get(ctx, companyID, instanceID)
	if err != nil {
		return err
	}
	if !settings.WebhookEnabled {
		return fmt.Errorf("%w: webhook is disabled for instance", ErrInvalidInstanceSettings)
	}
	if !hmac.Equal([]byte(settings.WebhookToken), []byte(strings.TrimSpace(token))) {
		return ErrInvalidToken
	}
	return nil
}

func (s *InstanceSettingsService) defaultSettings(companyID int64, instanceID string) instancecfg.Settings {
	now := time.Now().UTC()
	return instancecfg.Settings{
		CompanyID:            companyID,
		InstanceID:           instanceID,
		HumanizerEnabled:     s.humanizer.Enabled,
		InitialDelayMinSec:   int(s.humanizer.InitialDelayMin / time.Second),
		InitialDelayMaxSec:   int(s.humanizer.InitialDelayMax / time.Second),
		BaseDelayMinSec:      int(s.humanizer.BaseDelayMin / time.Second),
		BaseDelayMaxSec:      int(s.humanizer.BaseDelayMax / time.Second),
		ProviderDelayMinMS:   int(s.humanizer.ProviderDelayMin / time.Millisecond),
		ProviderDelayMaxMS:   int(s.humanizer.ProviderDelayMax / time.Millisecond),
		BurstSizeMin:         s.humanizer.BurstSizeMin,
		BurstSizeMax:         s.humanizer.BurstSizeMax,
		BurstPauseMinSec:     int(s.humanizer.BurstPauseMin / time.Second),
		BurstPauseMaxSec:     int(s.humanizer.BurstPauseMax / time.Second),
		WebhookEnabled:       true,
		WebhookSubscriptions: append([]string(nil), s.webhook.DefaultSubscriptions...),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func (s *InstanceSettingsService) enrich(item instancecfg.Settings) instancecfg.Settings {
	item.WebhookToken = s.webhookToken(item.CompanyID, item.InstanceID)
	if s.app.PublicURL != "" {
		item.WebhookURL = fmt.Sprintf(
			"%s/api/v1/webhooks/evolution/%d/%s?token=%s",
			s.app.PublicURL,
			item.CompanyID,
			item.InstanceID,
			item.WebhookToken,
		)
	}
	if len(item.WebhookSubscriptions) == 0 {
		item.WebhookSubscriptions = append([]string(nil), s.webhook.DefaultSubscriptions...)
	}
	return item
}

func (s *InstanceSettingsService) webhookToken(companyID int64, instanceID string) string {
	mac := hmac.New(sha256.New, []byte(s.webhook.TokenSecret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d:%s", companyID, instanceID)))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

func (s *InstanceSettingsService) validate(item *instancecfg.Settings) error {
	switch {
	case item.InitialDelayMinSec < 0,
		item.InitialDelayMaxSec < item.InitialDelayMinSec,
		item.BaseDelayMinSec < 0,
		item.BaseDelayMaxSec < item.BaseDelayMinSec,
		item.ProviderDelayMinMS < 0,
		item.ProviderDelayMaxMS < item.ProviderDelayMinMS,
		item.BurstSizeMin <= 0,
		item.BurstSizeMax < item.BurstSizeMin,
		item.BurstPauseMinSec < 0,
		item.BurstPauseMaxSec < item.BurstPauseMinSec:
		return fmt.Errorf("%w: invalid timing range", ErrInvalidInstanceSettings)
	}

	subscriptions := make([]string, 0, len(item.WebhookSubscriptions))
	for _, event := range item.WebhookSubscriptions {
		value := strings.ToUpper(strings.TrimSpace(event))
		if value != "" {
			subscriptions = append(subscriptions, value)
		}
	}
	if len(subscriptions) == 0 {
		subscriptions = append([]string(nil), s.webhook.DefaultSubscriptions...)
	}
	item.WebhookSubscriptions = subscriptions
	return nil
}
