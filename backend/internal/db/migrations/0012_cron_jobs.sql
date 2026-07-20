-- Cron job genérico: roda um comando shell dentro de um container num
-- horário/intervalo. "Cron básico" de propósito, mesmo espírito do
-- backup_policies (0005) — sem parser de expressão cron de verdade, só
-- intervalo em minutos OU diário/semanal + horário.
CREATE TABLE IF NOT EXISTS cron_jobs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    container_id     TEXT NOT NULL,
    container_name   TEXT NOT NULL DEFAULT '', -- só exibição, nunca usado pra rodar
    name             TEXT NOT NULL,
    command          TEXT NOT NULL, -- roda via `sh -c <command>` dentro do container
    frequency        TEXT NOT NULL, -- 'interval' | 'daily' | 'weekly'
    interval_minutes INTEGER,
    weekday          INTEGER, -- 0=domingo..6=sábado, só usado se frequency='weekly'
    time_of_day      TEXT NOT NULL DEFAULT '00:00', -- 'HH:MM' UTC, usado por daily/weekly
    enabled          BOOLEAN NOT NULL DEFAULT true,
    last_run_at      TIMESTAMPTZ,
    last_exit_code   INTEGER,
    last_output      TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cron_jobs_container ON cron_jobs (container_id);
