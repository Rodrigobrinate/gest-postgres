CREATE TABLE IF NOT EXISTS backup_policies (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id        UUID NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    database_name    TEXT NOT NULL,
    storage          TEXT NOT NULL, -- 'local' | 'gdrive'
    frequency        TEXT NOT NULL, -- 'daily' | 'weekly'
    weekday          INTEGER, -- 0=domingo..6=sábado, só usado se frequency='weekly'
    time_of_day      TEXT NOT NULL, -- 'HH:MM', formato 24h
    retention_count  INTEGER NOT NULL DEFAULT 7,
    enabled          BOOLEAN NOT NULL DEFAULT true,
    last_run_at      TIMESTAMPTZ,
    last_run_status  TEXT NOT NULL DEFAULT '',
    last_run_error   TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS backups (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id      UUID NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    policy_id      UUID REFERENCES backup_policies (id) ON DELETE SET NULL,
    database_name  TEXT NOT NULL,
    storage        TEXT NOT NULL, -- 'local' | 'gdrive'
    filename       TEXT NOT NULL,
    size_bytes     BIGINT,
    status         TEXT NOT NULL DEFAULT 'running', -- 'running' | 'completed' | 'failed'
    error          TEXT NOT NULL DEFAULT '',
    gdrive_file_id TEXT NOT NULL DEFAULT '',
    started_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_backups_server ON backups (server_id);
CREATE INDEX IF NOT EXISTS idx_backup_policies_server ON backup_policies (server_id);

-- Conexão Google Drive é da PLATAFORMA inteira, não por servidor — uma conta
-- só, todos os servidores que escolherem storage='gdrive' usam a mesma. Linha
-- única de propósito (sempre id=1), mais simples que multi-tenant de conta.
CREATE TABLE IF NOT EXISTS gdrive_connection (
    id                      INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    client_id               TEXT NOT NULL DEFAULT '',
    client_secret_encrypted TEXT NOT NULL DEFAULT '',
    refresh_token_encrypted TEXT NOT NULL DEFAULT '',
    account_email           TEXT NOT NULL DEFAULT '',
    folder_id               TEXT NOT NULL DEFAULT '',
    connected_at            TIMESTAMPTZ,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
