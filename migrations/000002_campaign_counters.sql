ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS pending_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS processing_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sent_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS failed_count INTEGER NOT NULL DEFAULT 0;

UPDATE campaigns c
SET pending_count = counts.pending_count,
    processing_count = counts.processing_count,
    sent_count = counts.sent_count,
    failed_count = counts.failed_count
FROM (
    SELECT
        campaign_id,
        COUNT(*) FILTER (WHERE status = 'pending') AS pending_count,
        COUNT(*) FILTER (WHERE status = 'processing') AS processing_count,
        COUNT(*) FILTER (WHERE status = 'sent') AS sent_count,
        COUNT(*) FILTER (WHERE status = 'failed') AS failed_count
    FROM campaign_messages
    GROUP BY campaign_id
) AS counts
WHERE c.id = counts.campaign_id;
