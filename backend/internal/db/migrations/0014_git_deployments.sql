-- Deploy automático via webhook: guarda a config de clone+build usada pelo
-- modo Git de "novo container" (ver git_build.go) de forma persistente, +
-- um segredo pra autenticar o POST que o GitHub/GitLab manda a cada push.
-- webhook_secret_encrypted cifrado com o mesmo internal/crypto.SecretBox de
-- sempre; devolvido em texto puro só uma vez, na criação.
CREATE TABLE IF NOT EXISTS git_deployments (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    container_name           TEXT NOT NULL UNIQUE,
    image_tag                TEXT NOT NULL,
    repo_url                 TEXT NOT NULL,
    branch                   TEXT NOT NULL DEFAULT 'main',
    credential_id            UUID REFERENCES git_credentials (id) ON DELETE SET NULL,
    env_json                 TEXT NOT NULL DEFAULT '{}',
    ports_json                TEXT NOT NULL DEFAULT '{}',
    network_name              TEXT NOT NULL DEFAULT '',
    webhook_secret_encrypted TEXT NOT NULL,
    last_deployed_at         TIMESTAMPTZ,
    last_status              TEXT NOT NULL DEFAULT '', -- '' | 'success' | 'failed'
    last_error               TEXT NOT NULL DEFAULT '',
    last_build_log           TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);
