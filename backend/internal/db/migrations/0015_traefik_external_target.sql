-- Rota de domínio apontando pra fora do Docker gerenciado por essa
-- plataforma (IP/host externo, serviço em outro lugar) — igual ao que
-- EasyPanel chama de "apontar pra outro destino". Modo extra, ao lado de
-- proxy-pra-container e redirect: os três são mutuamente exclusivos.
ALTER TABLE proxy_routes
    ADD COLUMN target_url TEXT NOT NULL DEFAULT '';

ALTER TABLE proxy_routes DROP CONSTRAINT IF EXISTS proxy_routes_target_or_redirect;

ALTER TABLE proxy_routes
    ADD CONSTRAINT proxy_routes_target_or_redirect CHECK (
        (target_container <> '' AND target_port > 0 AND redirect_target = '' AND target_url = '')
        OR (target_container = '' AND target_port = 0 AND redirect_target <> '' AND target_url = '')
        OR (target_container = '' AND target_port = 0 AND redirect_target = '' AND target_url <> '')
    );
