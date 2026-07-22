-- Gráfico "Leituras e escritas" na aba Monitoramento de cada servidor —
-- pedido explícito do usuário. Linha nova, não editar 0019: essa migration
-- já pode ter rodado em instalação existente (rastreada só por nome de
-- arquivo, ver internal/db/db.go), editar o arquivo aplicado não refaz nada.
--
-- avg/max, não só avg — mesmo raciocínio do resto de metric_history: pico
-- de leitura/escrita continua visível no resumo horário.
ALTER TABLE metric_history
    ADD COLUMN read_tuples_per_sec_avg  DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN read_tuples_per_sec_max  DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN write_tuples_per_sec_avg DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN write_tuples_per_sec_max DOUBLE PRECISION NOT NULL DEFAULT 0;
