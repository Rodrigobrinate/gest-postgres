-- Config da plataforma pra Traefik (reverse proxy + Let's Encrypt) — linha
-- única, mesmo padrão do gdrive_connection.
CREATE TABLE IF NOT EXISTS platform_settings (
    id                   INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    traefik_enabled      BOOLEAN NOT NULL DEFAULT false,
    traefik_container_id TEXT NOT NULL DEFAULT '',
    acme_email           TEXT NOT NULL DEFAULT '',
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Rotas de domínio → container:porta configuradas via Traefik (file
-- provider — ver internal/infra/traefik.go).
CREATE TABLE IF NOT EXISTS proxy_routes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain           TEXT NOT NULL UNIQUE,
    target_container TEXT NOT NULL,
    target_port      INTEGER NOT NULL,
    tls              BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
