ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS delivered_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS read_count INTEGER NOT NULL DEFAULT 0;

ALTER TABLE campaign_messages
    ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ;

UPDATE campaigns c
SET delivered_count = counts.delivered_count,
    read_count = counts.read_count
FROM (
    SELECT
        campaign_id,
        COUNT(*) FILTER (WHERE status = 'delivered') AS delivered_count,
        COUNT(*) FILTER (WHERE status = 'read') AS read_count
    FROM campaign_messages
    GROUP BY campaign_id
) AS counts
WHERE c.id = counts.campaign_id;
