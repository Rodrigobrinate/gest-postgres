-- Backup genérico (não-Postgres) de um volume nomeado qualquer — snapshot
-- .tar.gz manual, sem agendamento (diferente de backup_policies, que é só
-- pra servidor Postgres gerenciado via pg_dump). Arquivo fica em
-- generic_backups_data (ver docker-compose.yml), nunca bind mount do host,
-- mesmo raciocínio do resto do projeto.
CREATE TABLE IF NOT EXISTS volume_backups (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    volume_name  TEXT NOT NULL,
    filename     TEXT NOT NULL,
    size_bytes   BIGINT,
    status       TEXT NOT NULL DEFAULT 'running', -- 'running' | 'completed' | 'failed'
    error        TEXT NOT NULL DEFAULT '',
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_volume_backups_volume ON volume_backups (volume_name);
