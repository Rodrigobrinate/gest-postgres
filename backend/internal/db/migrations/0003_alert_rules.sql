CREATE TABLE IF NOT EXISTS alert_rules (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id            UUID NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    metric               TEXT NOT NULL, -- 'connections_pct' | 'disk_pct' | 'long_running_query_seconds' | 'deadlocks'
    threshold            DOUBLE PRECISION NOT NULL,
    webhook_url          TEXT NOT NULL,
    enabled              BOOLEAN NOT NULL DEFAULT true,
    last_triggered_at    TIMESTAMPTZ,
    last_value           DOUBLE PRECISION,
    last_deadlock_count  BIGINT NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_server ON alert_rules (server_id);
