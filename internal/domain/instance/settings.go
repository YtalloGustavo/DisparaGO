package instance

import "time"

type Settings struct {
	InstanceID           string    `json:"instance_id"`
	HumanizerEnabled     bool      `json:"humanizer_enabled"`
	InitialDelayMinSec   int       `json:"initial_delay_min_seconds"`
	InitialDelayMaxSec   int       `json:"initial_delay_max_seconds"`
	BaseDelayMinSec      int       `json:"base_delay_min_seconds"`
	BaseDelayMaxSec      int       `json:"base_delay_max_seconds"`
	ProviderDelayMinMS   int       `json:"provider_delay_min_ms"`
	ProviderDelayMaxMS   int       `json:"provider_delay_max_ms"`
	BurstSizeMin         int       `json:"burst_size_min"`
	BurstSizeMax         int       `json:"burst_size_max"`
	BurstPauseMinSec     int       `json:"burst_pause_min_seconds"`
	BurstPauseMaxSec     int       `json:"burst_pause_max_seconds"`
	WebhookEnabled       bool      `json:"webhook_enabled"`
	WebhookSubscriptions []string  `json:"webhook_subscriptions"`
	WebhookURL           string    `json:"webhook_url"`
	WebhookToken         string    `json:"webhook_token"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
