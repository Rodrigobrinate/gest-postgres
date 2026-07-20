-- Autenticação de admin único (sem RBAC multi-nível, fora do MVP) — sessão
-- guardada no banco de metadados pra sobreviver a restart do backend, não
-- num mapa em memória. Ver internal/auth. admin_user é singleton, mesmo
-- padrão do gdrive_connection/platform_settings.
CREATE TABLE IF NOT EXISTS admin_user (
    id            INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- token_hash guarda sha256(token) — o token em texto puro nunca é
-- persistido, só devolvido uma vez no login (vira o cookie httpOnly).
-- elevated_until é a reconfirmação de senha ("step-up") pra operações de
-- risco (ex: escrever/apagar no file manager do host) — vive na mesma linha
-- da sessão em vez de um token elevado separado, mais simples.
CREATE TABLE IF NOT EXISTS admin_sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash     TEXT NOT NULL UNIQUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    last_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    elevated_until TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS admin_sessions_expires_at_idx ON admin_sessions (expires_at);
