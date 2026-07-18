-- PgBouncer opcional por servidor: um container companheiro, criado/removido
-- sob demanda (não sobe junto com o Postgres por padrão, é um toggle).
ALTER TABLE servers ADD COLUMN IF NOT EXISTS pooler_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE servers ADD COLUMN IF NOT EXISTS pooler_container_id TEXT NOT NULL DEFAULT '';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS pooler_container_name TEXT NOT NULL DEFAULT '';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS pooler_host_port INTEGER;
ALTER TABLE servers ADD COLUMN IF NOT EXISTS pooler_pool_mode TEXT NOT NULL DEFAULT 'transaction';

-- Único só entre portas realmente em uso (parcial, porque a maioria dos
-- servidores nunca terá pooling habilitado e todos comparariam NULL = NULL
-- como não-conflitante de qualquer forma, mas fica explícito).
CREATE UNIQUE INDEX IF NOT EXISTS idx_servers_pooler_host_port ON servers (pooler_host_port) WHERE pooler_host_port IS NOT NULL;
