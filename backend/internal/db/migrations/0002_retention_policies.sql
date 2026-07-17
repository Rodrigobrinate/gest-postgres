CREATE TABLE IF NOT EXISTS retention_policies (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id             UUID NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    database_name         TEXT NOT NULL,
    schema_name           TEXT NOT NULL,
    table_name            TEXT NOT NULL,
    date_column           TEXT NOT NULL,
    max_age_days          INTEGER NOT NULL,
    action                TEXT NOT NULL, -- 'archive' | 'delete'
    enabled               BOOLEAN NOT NULL DEFAULT true,
    last_run_at           TIMESTAMPTZ,
    last_run_rows_affected INTEGER,
    last_run_error        TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_retention_policies_server ON retention_policies (server_id);
