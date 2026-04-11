CREATE TABLE IF NOT EXISTS campaigns (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    message_content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    total_messages INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS campaign_messages (
    id UUID PRIMARY KEY,
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    recipient_phone TEXT NOT NULL,
    message_content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    provider_message_id TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    sent_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_campaign_messages_campaign_id
    ON campaign_messages (campaign_id);

CREATE INDEX IF NOT EXISTS idx_campaign_messages_status
    ON campaign_messages (status);
