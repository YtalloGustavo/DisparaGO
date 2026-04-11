CREATE TABLE IF NOT EXISTS instance_settings (
    instance_id TEXT PRIMARY KEY,
    humanizer_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    initial_delay_min_seconds INTEGER NOT NULL DEFAULT 3,
    initial_delay_max_seconds INTEGER NOT NULL DEFAULT 8,
    base_delay_min_seconds INTEGER NOT NULL DEFAULT 9,
    base_delay_max_seconds INTEGER NOT NULL DEFAULT 18,
    provider_delay_min_ms INTEGER NOT NULL DEFAULT 1500,
    provider_delay_max_ms INTEGER NOT NULL DEFAULT 5000,
    burst_size_min INTEGER NOT NULL DEFAULT 4,
    burst_size_max INTEGER NOT NULL DEFAULT 8,
    burst_pause_min_seconds INTEGER NOT NULL DEFAULT 45,
    burst_pause_max_seconds INTEGER NOT NULL DEFAULT 120,
    webhook_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    webhook_subscriptions TEXT[] NOT NULL DEFAULT ARRAY['ALL']::TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
