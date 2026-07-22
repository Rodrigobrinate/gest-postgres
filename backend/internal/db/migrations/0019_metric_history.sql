-- Persistência de métricas (CPU/memória/conexões/disco/rede) — antes só em
-- memória (HistoryCollector/platformHistory), zerava em QUALQUER reinício do
-- backend (update, restart manual, deploy comum), não só update. Pedido
-- explícito do usuário depois de perder histórico ao atualizar.
--
-- Duas resoluções na mesma tabela, não duas tabelas: "raw" (uma linha por
-- amostra, ~15s, só as últimas ~24h) e "hourly" (uma linha por hora,
-- avg/min/max, retenção bem mais longa) — job de rollup em background
-- agrega raw>24h em hourly e apaga o raw já agregado, mantém a tabela
-- limitada em vez de crescer pra sempre. min/max (não só avg) é o que
-- deixa dar pra ver "bateu um pico nesse dia" mesmo depois do dado raw ter
-- sido resumido — média sozinha esconderia justamente o pico que importa.
CREATE TABLE IF NOT EXISTS metric_history (
    id                       BIGSERIAL PRIMARY KEY,
    scope                    TEXT NOT NULL CHECK (scope IN ('platform', 'server')),
    server_id                UUID REFERENCES servers(id) ON DELETE CASCADE,
    resolution               TEXT NOT NULL CHECK (resolution IN ('raw', 'hourly')),
    bucket_start             TIMESTAMPTZ NOT NULL,
    cpu_percent_avg          DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_percent_min          DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_percent_max          DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_mb_avg       DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_mb_min       DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_mb_max       DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_limit_mb          DOUBLE PRECISION NOT NULL DEFAULT 0,
    connection_count_avg     DOUBLE PRECISION NOT NULL DEFAULT 0,
    connection_count_max     INTEGER NOT NULL DEFAULT 0,
    disk_used_mb_avg         DOUBLE PRECISION NOT NULL DEFAULT 0,
    disk_used_mb_max         DOUBLE PRECISION NOT NULL DEFAULT 0,
    network_rx_bytes         BIGINT NOT NULL DEFAULT 0,
    network_tx_bytes         BIGINT NOT NULL DEFAULT 0,
    -- Só platform (I/O é medido no host inteiro via /proc/diskstats, não
    -- por servidor) — operações/segundo, não bytes, pedido explícito
    -- comparando com um painel Zabbix noutro host.
    read_ops_per_sec_avg     DOUBLE PRECISION NOT NULL DEFAULT 0,
    read_ops_per_sec_max     DOUBLE PRECISION NOT NULL DEFAULT 0,
    write_ops_per_sec_avg    DOUBLE PRECISION NOT NULL DEFAULT 0,
    write_ops_per_sec_max    DOUBLE PRECISION NOT NULL DEFAULT 0,
    -- Só preenchido em scope='server' resolution='raw' — o breakdown por
    -- banco (gráficos "Disco por banco"/"Conexões por banco") não entra no
    -- rollup horário, só o agregado; o detalhe por banco fica só na janela
    -- recente, mesmo trade-off que "não preciso de tudo enchendo o banco".
    database_sizes_mb        JSONB,
    connections_by_database  JSONB,
    CONSTRAINT metric_history_scope_server_id CHECK (
        (scope = 'server' AND server_id IS NOT NULL) OR
        (scope = 'platform' AND server_id IS NULL)
    )
);

-- Índices parciais (não um único índice com server_id nas colunas) porque
-- unicidade comum trata NULL como "distinto de qualquer outro NULL" — sem
-- isso, scope='platform' (sempre server_id NULL) nunca barraria linha
-- duplicada no mesmo bucket_start via UNIQUE normal.
CREATE UNIQUE INDEX IF NOT EXISTS metric_history_platform_uniq
    ON metric_history (resolution, bucket_start) WHERE scope = 'platform';
CREATE UNIQUE INDEX IF NOT EXISTS metric_history_server_uniq
    ON metric_history (server_id, resolution, bucket_start) WHERE scope = 'server';

-- Consulta principal é sempre "esse server/platform, essa resolução, nessa
-- janela de tempo, em ordem" — os índices únicos acima já cobrem isso (o
-- prefixo bate com o padrão de busca), sem precisar de um terceiro índice.
