# Monitoramento

Aba **Monitoramento** de cada servidor. (Dashboard geral da plataforma, com todos os containers, fica em [Plataforma](plataforma.md#dashboard-principal).)

## `pg_stat_activity` ao vivo

Sessões, query atual, estado — com botão pra cancelar/terminar sessão.

## Gráficos

- **Conexões** ao longo do tempo.
- **CPU** e **memória** do container (via `docker stats`).
- **Disco** — 4º gráfico, soma de `pg_database_size` de todo banco não-template.

Histórico em memória (goroutine de coleta a cada ~15s) — **reseta se o backend reiniciar**, sem armazenamento de longo prazo no MVP.

## Disco por banco

Gráfico de linhas "Disco por banco", pedido explícito fora do MVP original (usuário pediu como "memória por banco" — Postgres não rastreia RAM por banco individual, `shared_buffers` é um pool só do cluster inteiro; o proxy mais próximo é tamanho em disco via `pg_database_size`, deixado explícito na UI). Uma linha colorida por banco; banco criado/excluído no meio da janela não quebra a linha (`connectNulls`).

## Zoom nos gráficos

Todo gráfico é clicável — abre modal ampliado com botões de período (5min/15min/30min/tudo). Isso **recorta o mesmo buffer em memória** já coletado — não busca dado mais antigo no backend, já que não existe armazenamento de métricas de longo prazo no MVP.

## Databases

Lista de databases com tamanho — ver [Banco de dados e objetos](banco-de-dados.md#databases).

## Top queries lentas

Ver aba **Desempenho**, documentada em [Banco de dados e objetos](banco-de-dados.md#desempenho).
