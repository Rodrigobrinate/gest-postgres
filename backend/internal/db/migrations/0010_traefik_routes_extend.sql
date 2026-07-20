-- Rotas de proxy ganham path/redirect/protocolo mais ricos (ver
-- internal/infra/traefik.go writeDynamicConfig). Sentinelas vazio/0 em vez
-- de colunas nullable pra manter o estilo das colunas já existentes.
ALTER TABLE proxy_routes
    ADD COLUMN path_prefix        TEXT NOT NULL DEFAULT '/',
    ADD COLUMN strip_prefix       BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN redirect_target    TEXT NOT NULL DEFAULT '',
    ADD COLUMN redirect_permanent BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN https_redirect     BOOLEAN NOT NULL DEFAULT false;

-- Uma rota é OU um proxy pra container:porta OU um redirect — nunca as
-- duas coisas ao mesmo tempo, nem nenhuma das duas.
ALTER TABLE proxy_routes
    ADD CONSTRAINT proxy_routes_target_or_redirect CHECK (
        (target_container <> '' AND target_port > 0 AND redirect_target = '')
        OR (target_container = '' AND target_port = 0 AND redirect_target <> '')
    );
