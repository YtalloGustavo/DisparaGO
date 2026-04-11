ALTER TABLE campaign_messages
    ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;
