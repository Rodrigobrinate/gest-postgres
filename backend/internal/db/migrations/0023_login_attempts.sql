-- Tela de "Gestão de sessões" (pedido explícito do usuário, 2026-07-23):
-- usuários logados, online, tentativas de login, histórico de sessão.
--
-- login_attempts guarda o username DIGITADO (não FK pra users — username
-- errado/inexistente também é uma tentativa que precisa aparecer no log de
-- segurança), sucesso ou falha, e a origem. Alimentado pelo mesmo Login()
-- que já faz o throttle em memória (internal/auth/login.go) — esse throttle
-- continua em memória de propósito (decisão só de rate-limit), essa tabela
-- é o log durável, só pra auditoria/visualização.
CREATE TABLE IF NOT EXISTS login_attempts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username   TEXT NOT NULL,
    success    BOOLEAN NOT NULL,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS login_attempts_created_at_idx ON login_attempts (created_at DESC);

-- admin_sessions ganha rastro de origem (IP/user-agent no momento do
-- login) e revoked_at — sessão de logout/expirada agora fica registrada em
-- vez de simplesmente apagada (DELETE virou UPDATE revoked_at), pra virar
-- histórico de sessão de verdade. RunSessionRetentionSweep (novo) apaga só
-- o que já está encerrado (revoked_at ou expires_at no passado) há mais de
-- 90 dias — sessão ATIVA nunca é tocada por esse sweep.
ALTER TABLE admin_sessions ADD COLUMN IF NOT EXISTS ip_address TEXT;
ALTER TABLE admin_sessions ADD COLUMN IF NOT EXISTS user_agent TEXT;
ALTER TABLE admin_sessions ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;
