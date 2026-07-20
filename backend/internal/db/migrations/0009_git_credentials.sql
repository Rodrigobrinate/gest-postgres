-- Credenciais pra clonar repositório Git privado (aba Docker > Novo
-- container > Git). Não é singleton — várias credenciais permitidas.
-- secret_encrypted guarda a chave SSH privada (kind='ssh_key') ou o PAT
-- (kind='pat'), cifrado com o mesmo internal/crypto.SecretBox usado pra
-- senha de servidor/refresh_token do Drive.
CREATE TABLE IF NOT EXISTS git_credentials (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL UNIQUE,
    kind             TEXT NOT NULL CHECK (kind IN ('ssh_key', 'pat')),
    username         TEXT NOT NULL DEFAULT '',
    secret_encrypted TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
