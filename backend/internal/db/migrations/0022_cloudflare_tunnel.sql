-- Config do container cloudflared (Cloudflare Tunnel) — mesma forma de
-- traefik_enabled/traefik_container_id (0007_traefik.sql), tabela já
-- existente de config singleton da plataforma. Token cifrado com o mesmo
-- secretBox usado pra todo outro segredo salvo (gdrive, git PAT, etc).
ALTER TABLE platform_settings
    ADD COLUMN IF NOT EXISTS cloudflare_tunnel_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS cloudflare_tunnel_token_encrypted TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cloudflare_tunnel_container_id TEXT NOT NULL DEFAULT '';
