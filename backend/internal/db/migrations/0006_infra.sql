-- Stacks implantados via docker-compose.yml pela tela de "Docker".
CREATE TABLE IF NOT EXISTS compose_projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    compose_content TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'deployed', -- 'deployed' | 'error'
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
