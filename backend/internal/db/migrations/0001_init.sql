-- Banco de METADADOS da plataforma. Não confundir com os Postgres gerenciados
-- que os usuários criam pela UI — esse aqui só guarda o registro deles.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS servers (
    id                            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                          TEXT NOT NULL UNIQUE,
    description                   TEXT NOT NULL DEFAULT '',
    version                       TEXT NOT NULL,
    status                        TEXT NOT NULL DEFAULT 'creating',
    preset                        TEXT NOT NULL,

    cpu_cores                     DOUBLE PRECISION NOT NULL,
    memory_mb                     INTEGER NOT NULL,
    disk_gb                       INTEGER NOT NULL,

    max_connections               INTEGER NOT NULL,
    shared_buffers_mb             INTEGER NOT NULL,
    work_mem_mb                   INTEGER NOT NULL,
    maintenance_work_mem_mb       INTEGER NOT NULL,
    effective_cache_size_mb       INTEGER NOT NULL,
    log_min_duration_statement_ms INTEGER NOT NULL,

    host_port                     INTEGER NOT NULL UNIQUE,
    username                      TEXT NOT NULL,
    password_encrypted            TEXT NOT NULL,
    database_name                 TEXT NOT NULL,

    container_id                  TEXT NOT NULL DEFAULT '',
    container_name                TEXT NOT NULL,
    volume_name                   TEXT NOT NULL,

    created_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_servers_status ON servers (status);
