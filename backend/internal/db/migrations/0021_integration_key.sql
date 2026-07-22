-- Chave de integração: credencial bearer pro futuro sistema "mestre" (Worker
-- na Cloudflare) chamar essa API sem sessão de cookie — ver
-- internal/auth/integration_key.go. Só o hash fica salvo (mesmo raciocínio
-- de admin_sessions.token_hash), texto puro é mostrado uma vez só na rotação.
CREATE TABLE IF NOT EXISTS integration_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash     TEXT NOT NULL UNIQUE,
    label        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

-- Índice único parcial numa expressão constante: garante em nível de banco
-- que só existe 1 chave ATIVA por vez (revoked_at IS NULL) — rotação precisa
-- ser revoke+insert na mesma transação, não só disciplina de aplicação.
-- Mantém histórico de chaves revogadas (auditoria) em vez de sobrescrever
-- uma linha só.
CREATE UNIQUE INDEX IF NOT EXISTS integration_keys_one_active_idx
    ON integration_keys ((true)) WHERE revoked_at IS NULL;
