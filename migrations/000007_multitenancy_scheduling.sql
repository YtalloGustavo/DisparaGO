CREATE TABLE IF NOT EXISTS auth_companies (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    external_source TEXT,
    external_source_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_companies_external_ref
    ON auth_companies (external_source, external_source_id)
    WHERE external_source IS NOT NULL AND external_source_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS auth_users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT REFERENCES auth_companies (id) ON DELETE CASCADE,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    external_source TEXT,
    external_source_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_users_external_ref
    ON auth_users (external_source, external_source_id)
    WHERE external_source IS NOT NULL AND external_source_id IS NOT NULL;

INSERT INTO auth_companies (name, external_source, external_source_id)
SELECT 'Default Company', 'bootstrap', 'default'
WHERE NOT EXISTS (
    SELECT 1
    FROM auth_companies
    WHERE external_source = 'bootstrap' AND external_source_id = 'default'
);

ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS company_id BIGINT,
    ADD COLUMN IF NOT EXISTS created_by_user_id BIGINT,
    ADD COLUMN IF NOT EXISTS send_mode TEXT NOT NULL DEFAULT 'now',
    ADD COLUMN IF NOT EXISTS scheduled_at_utc TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS scheduled_timezone TEXT,
    ADD COLUMN IF NOT EXISTS scheduled_original_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS released_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS external_source TEXT,
    ADD COLUMN IF NOT EXISTS external_source_id TEXT;

UPDATE campaigns
SET company_id = (
    SELECT id
    FROM auth_companies
    WHERE external_source = 'bootstrap' AND external_source_id = 'default'
)
WHERE company_id IS NULL;

ALTER TABLE campaigns
    ALTER COLUMN company_id SET NOT NULL;

ALTER TABLE campaigns
    ADD CONSTRAINT campaigns_company_id_fkey
    FOREIGN KEY (company_id) REFERENCES auth_companies (id) ON DELETE CASCADE;

ALTER TABLE campaigns
    ADD CONSTRAINT campaigns_created_by_user_id_fkey
    FOREIGN KEY (created_by_user_id) REFERENCES auth_users (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_campaigns_company_created_at
    ON campaigns (company_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_campaigns_company_status
    ON campaigns (company_id, status);

CREATE INDEX IF NOT EXISTS idx_campaigns_scheduled_lookup
    ON campaigns (status, scheduled_at_utc)
    WHERE released_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_campaigns_external_ref
    ON campaigns (external_source, external_source_id)
    WHERE external_source IS NOT NULL AND external_source_id IS NOT NULL;

ALTER TABLE instance_settings
    ADD COLUMN IF NOT EXISTS company_id BIGINT;

UPDATE instance_settings
SET company_id = (
    SELECT id
    FROM auth_companies
    WHERE external_source = 'bootstrap' AND external_source_id = 'default'
)
WHERE company_id IS NULL;

ALTER TABLE instance_settings
    ALTER COLUMN company_id SET NOT NULL;

ALTER TABLE instance_settings
    ADD CONSTRAINT instance_settings_company_id_fkey
    FOREIGN KEY (company_id) REFERENCES auth_companies (id) ON DELETE CASCADE;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'instance_settings'::regclass
          AND conname = 'instance_settings_pkey'
    ) THEN
        ALTER TABLE instance_settings DROP CONSTRAINT instance_settings_pkey;
    END IF;
END $$;

ALTER TABLE instance_settings
    ADD PRIMARY KEY (company_id, instance_id);

CREATE INDEX IF NOT EXISTS idx_instance_settings_instance_id
    ON instance_settings (instance_id);
