# Configuração

Aba **Configuração** de cada servidor. Cobre ~86 parâmetros do `postgresql.conf`, agrupados por categoria (memória, conexões, WAL, autovacuum, logging, etc.), com busca, indicação de quais pedem restart vs. reload, e edição só dos campos alterados.

## Como a mudança é aplicada

Todo parâmetro — tanto na criação do servidor quanto em qualquer edição depois — passa por `ALTER SYSTEM SET` seguido de `pg_reload_conf()` (parâmetros que aceitam reload) ou aviso de restart necessário (parâmetros que não aceitam). Nunca via flag `-c` no comando do container.

> Isso corrigiu um bug de arquitetura sério do início do projeto: com a config inicial entrando via flag `-c`, que tem prioridade **maior** que `ALTER SYSTEM`, nenhuma edição pós-criação surtia efeito — nem com restart. Ver [Arquitetura](arquitetura.md#fluxo-de-configuração-do-postgres).

## Modo Simples vs. Avançado

- **Simples** — preset de recursos (Pequeno/Médio/Grande, escolhido na criação do servidor) calcula sozinho: `max_connections`, `shared_buffers`, `work_mem`, `maintenance_work_mem`, `effective_cache_size`, `log_min_duration_statement`.
- **Avançado** — os ~86 parâmetros geridos, um por um.

## O que fica fora do editável (de propósito)

| Parâmetro | Por quê |
|---|---|
| `listen_addresses`, `port`, `unix_socket_directories` | Mudar quebra o mecanismo de conexão da própria plataforma (que fala pelo nome do container) |
| certificados TLS | Sem orquestração de emissão/renovação no MVP |
| `shared_preload_libraries` | Editado só indiretamente (habilitar `pg_stat_statements`/`pg_cron` cuida disso sozinho — ver [Extensões](extensoes.md)) |
| `recovery_target_*`, `restore_command` | Exigiria orquestração de recovery que a plataforma ainda não faz |
| toggles de debug (`enable_*`) | Mudar isso tipicamente piora, não ajuda — fora de escopo de uma UI de administração |

## `pg_hba.conf`

Tabela simples de regras (tipo, database, user, CIDR, método) — sem drag-and-drop.

- Lida/escrita **de dentro do container via API de archive do Docker** (`GET`/`PUT /containers/{id}/archive`) — não é `exec`, superfície de ataque bem menor.
- Recarrega via `pg_reload_conf()`, sem restart.
- Regra nova sempre vai pro **final** do arquivo — nunca esconde uma regra mais restritiva já existente antes dela.
- Sintaxe inválida faz o Postgres logar erro e manter as regras antigas em memória — não derruba conexão nem trava o servidor.
- Campos são validados contra espaço/tab embutido (evita smuggling de token extra na linha).

## Connection pooling

PgBouncer companheiro fica no mesmo card de Configuração — ver página própria: [Connection pooling](pooling.md).
