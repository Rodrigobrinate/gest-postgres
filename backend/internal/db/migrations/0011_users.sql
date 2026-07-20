-- Multi-usuário com 2 papéis (admin/viewer) — substitui o singleton
-- admin_user (migration 0008) por uma tabela de verdade. viewer é
-- read-only em tudo (aplicado via regra genérica no middleware: qualquer
-- método não-GET exige admin — ver internal/api/middleware.go), sem
-- permissão granular por servidor/container (fora de escopo por ora).
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin', 'viewer')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO users (username, password_hash, role)
SELECT username, password_hash, 'admin' FROM admin_user
ON CONFLICT (username) DO NOTHING;

ALTER TABLE admin_sessions ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Sessão criada antes dessa migration não tem user_id — associa com o
-- admin migrado (só existe 1 nesse ponto) ou derruba a sessão (login de
-- novo é mais simples que tentar adivinhar dono).
UPDATE admin_sessions SET user_id = (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1) WHERE user_id IS NULL;
DELETE FROM admin_sessions WHERE user_id IS NULL;
ALTER TABLE admin_sessions ALTER COLUMN user_id SET NOT NULL;

DROP TABLE admin_user;
