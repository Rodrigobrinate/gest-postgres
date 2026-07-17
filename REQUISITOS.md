# Requisitos do Sistema — Plataforma de Gestão PostgreSQL (Docker)

> Objetivo: ser a plataforma mais completa do mercado para provisionar, configurar, monitorar, proteger e fazer backup de instâncias PostgreSQL rodando em Docker — com interface amigável para quem não tem conhecimento técnico, mas com 100% de personalização exposta para quem tem.

---

## 0. Princípios de produto

1. **Dois modos de uso convivendo na mesma tela**: modo **Simples** (wizards, presets, tooltips explicando em português claro o que cada opção faz, valores recomendados) e modo **Avançado** (acesso bruto a todos os parâmetros, editor de arquivo de config, overrides livres). O usuário troca de modo a qualquer momento sem perder contexto.
2. **Nada fica escondido**: toda configuração que o PostgreSQL aceita deve estar acessível pela UI, mesmo que agrupada/simplificada no modo Simples.
3. **Nunca aplicar mudança perigosa sem avisar**: parâmetros que exigem restart do container (`context = postmaster`) devem ser sinalizados com badge diferente de parâmetros que só exigem `reload` (`context = sighup`) ou que são aplicáveis em runtime (`context = user/superuser`).
4. **Zero downtime perceptível** nas operações que permitem (reload de config, criação de índice `CONCURRENTLY`, etc.) — a UI deve preferir a via não-bloqueante quando existir.
5. **Auditável**: toda ação destrutiva ou de configuração fica registrada (quem, quando, o quê, diff antes/depois).
6. **Seguro por padrão**: senhas geradas fortes, SSL habilitado por padrão, backups criptografados, acesso ao Docker socket nunca direto.

---

## 1. Personas

- **Ana, iniciante**: sabe que precisa de "um banco Postgres pro projeto", não sabe o que é `shared_buffers`. Quer clicar em "Criar servidor", escolher um preset ("Pequeno/Médio/Grande") e pronto.
- **Bruno, DBA/dev sênior**: quer afinar `work_mem`, `random_page_cost`, ver `pg_stat_statements`, configurar replicação lógica, editar `pg_hba.conf` linha a linha.
- **Cíntia, operações**: só quer garantir que backup rodou, que não vai faltar disco, e receber alerta se algo cair.

---

## 2. Dashboard principal

Tela inicial ao logar, com visão consolidada de todos os servidores.

### 2.1 Cards de resumo (topo)
- Total de servidores (rodando / parados / com erro)
- Total de conexões ativas (soma de todos os servidores)
- Uso agregado de CPU / RAM / Disco do host
- Backups: último sucesso, próximos agendados, falhas nas últimas 24h
- Alertas ativos (contador por severidade: crítico / aviso / info)

### 2.2 Gráficos
- **Conexões ao longo do tempo** (linha, por servidor, últimas 1h/6h/24h/7d/30d — seletor de período)
- **CPU e RAM por servidor** (linha/área, empilhado ou por servidor)
- **IOPS / throughput de disco** por servidor (linha)
- **Cache hit ratio** por servidor (gauge ou linha, com linha de referência em 99%)
- **Transações por segundo** (commits vs rollbacks, linha)
- **Tamanho dos bancos ao longo do tempo** (linha de crescimento, para prever necessidade de disco)
- **Top 5 queries mais lentas** (barra horizontal, últimas 24h, vindo do `pg_stat_statements`)
- **Replicação**: lag em segundos/bytes por réplica (linha, se houver réplicas)
- **Status de backups** (calendário/heatmap tipo GitHub contributions: verde = sucesso, vermelho = falha, cinza = não rodou)
- **Mapa de servidores** (grid de cards, cada card = 1 servidor com status colorido, mini-sparkline de CPU/conexões, botões rápidos start/stop/restart)

### 2.3 Feed de eventos recentes
Lista cronológica: "Backup do servidor X concluído", "Servidor Y reiniciado por Ana", "Alerta: disco do servidor Z acima de 85%", "Extensão pg_trgm ativada em W".

### 2.4 Filtros globais
Filtrar dashboard por: servidor específico, grupo/tag de servidores, ambiente (produção/homologação/dev), período de tempo.

---

## 3. Gestão de servidores (ciclo de vida)

### 3.1 Criar servidor (wizard)
Passo a passo, com todos os campos já pré-preenchidos com defaults sensatos:

1. **Identificação**: nome do servidor, descrição, ambiente (produção/homologação/dev/teste), tags livres, grupo/projeto.
2. **Versão e imagem**: dropdown com versões do PostgreSQL suportadas (12, 13, 14, 15, 16, 17 — manter atualizado), escolha de imagem base (postgres oficial, ou imagem customizada da plataforma com extensões pré-instaladas), locale/encoding do cluster (`UTF8`, `pt_BR.UTF-8`, etc.).
3. **Credenciais**: usuário superuser (default `postgres`), senha (gerador automático de senha forte com opção de copiar, ou senha manual com medidor de força), nome do banco inicial.
4. **Recursos** (com presets): 
   - Preset **Pequeno** (1 vCPU / 1GB RAM / 10GB disco) — dev/teste
   - Preset **Médio** (2 vCPU / 4GB RAM / 50GB disco) — produção pequena
   - Preset **Grande** (4 vCPU / 16GB RAM / 200GB disco) — produção
   - Preset **Customizado**: sliders/inputs livres para CPU limit, memory limit, tamanho do volume de disco, tipo de storage (se aplicável).
   - Ao escolher um preset, os parâmetros de `postgresql.conf` relacionados a memória (`shared_buffers`, `effective_cache_size`, `work_mem`, `maintenance_work_mem`) são calculados automaticamente seguindo boas práticas (ex.: `shared_buffers` = 25% da RAM), mas ficam editáveis.
5. **Rede**: porta externa exposta, rede Docker (criar isolada por padrão ou anexar a rede existente), habilitar/desabilitar acesso externo, whitelist de IPs inicial.
6. **Configuração inicial do Postgres**: aqui entra a tela completa de parâmetros (ver seção 4), já preenchida com o que o preset calculou, 100% editável antes mesmo de criar o servidor.
7. **Extensões iniciais**: checklist de extensões para já vir habilitadas (ver seção 6).
8. **Backup**: oferecer configurar já uma rotina de backup (opcional, pode pular e configurar depois).
9. **Revisão final**: resumo de tudo, com opção "Ver docker-compose/comando equivalente" (transparência técnica para quem quiser auditar o que vai ser executado).
10. **Criar**: barra de progresso mostrando etapas (pull da imagem → criação de volume → criação de rede → start do container → healthcheck → aplicação de config inicial → pronto).

### 3.2 Listar servidores
Tabela/grid com: nome, status (rodando/parado/reiniciando/erro/criando), versão, ambiente, CPU/RAM atual, conexões atuais, uptime, tamanho total em disco, tags. Busca, filtro por status/ambiente/tag, ordenação por qualquer coluna.

### 3.3 Ações por servidor
- Start / Stop / Restart (com confirmação para produção)
- Editar (nome, descrição, tags, recursos, rede)
- Clonar servidor (cria novo servidor a partir de backup do atual)
- Fazer upgrade de versão maior (wizard de `pg_upgrade`, com backup automático antes, checagem de compatibilidade de extensões, rollback se falhar)
- Redimensionar recursos (CPU/RAM/disco) a quente quando possível, ou agendar para próxima janela de manutenção
- Excluir servidor (confirmação dupla, digitar nome do servidor para confirmar, opção "fazer backup final antes de excluir", opção de manter ou apagar volumes)
- Ver "docker inspect" / detalhes técnicos do container
- Terminal/console de logs do container (stdout/stderr do Docker, diferente dos logs internos do Postgres)

### 3.4 Editar servidor
Tudo que foi definido na criação deve poder ser alterado depois: recursos, rede, tags, configuração completa do Postgres, extensões, regras de acesso, rotina de backup.

---

## 4. Configuração completa do PostgreSQL (`postgresql.conf`)

Tela dedicada, organizada em abas por categoria (mesma organização usada pela documentação oficial do Postgres). Cada parâmetro exibe: nome técnico, nome amigável, descrição em português, valor atual, valor default, valor recomendado (calculado conforme recursos do servidor), unidade, `context` (badge: **runtime** / **precisa reload** / **precisa restart**), link para documentação oficial.

No modo Simples, os parâmetros mais avançados ficam colapsados atrás de "Mostrar configurações avançadas". No modo Avançado, tudo aparece, mais um **editor de texto puro** do `postgresql.conf` completo com syntax highlighting, validação de sintaxe antes de salvar, e diff antes de aplicar.

### 4.1 Conexões e Autenticação
- `listen_addresses`
- `port`
- `max_connections`
- `superuser_reserved_connections`
- `unix_socket_directories`, `unix_socket_group`, `unix_socket_permissions`
- `ssl`, `ssl_cert_file`, `ssl_key_file`, `ssl_ca_file`, `ssl_crl_file`, `ssl_ciphers`, `ssl_min_protocol_version`
- `password_encryption` (scram-sha-256 / md5)
- `authentication_timeout`

### 4.2 Consumo de recursos — Memória
- `shared_buffers`
- `huge_pages`
- `temp_buffers`
- `work_mem`
- `maintenance_work_mem`
- `autovacuum_work_mem`
- `max_stack_depth`
- `dynamic_shared_memory_type`
- `shared_memory_type`
- `vacuum_buffer_usage_limit`

### 4.3 Consumo de recursos — Disco
- `temp_file_limit`
- `max_files_per_process`

### 4.4 Consumo de recursos — Kernel/Processos
- `max_worker_processes`
- `effective_io_concurrency`
- `maintenance_io_concurrency`

### 4.5 Write Ahead Log (WAL)
- `wal_level` (minimal / replica / logical)
- `fsync`
- `synchronous_commit`
- `wal_sync_method`
- `full_page_writes`
- `wal_compression`
- `wal_buffers`
- `wal_writer_delay`, `wal_writer_flush_after`
- `wal_skip_threshold`
- `commit_delay`, `commit_siblings`
- `checkpoint_timeout`
- `checkpoint_completion_target`
- `checkpoint_flush_after`
- `checkpoint_warning`
- `max_wal_size`, `min_wal_size`
- `archive_mode`, `archive_command`, `archive_library`, `archive_timeout`
- `restore_command` (para réplicas/recovery)

### 4.6 Replicação
- `max_wal_senders`
- `max_replication_slots`
- `wal_keep_size`
- `wal_sender_timeout`, `wal_receiver_timeout`
- `hot_standby`
- `hot_standby_feedback`
- `max_standby_archive_delay`, `max_standby_streaming_delay`
- `synchronous_standby_names`
- `synchronous_commit` (remote_write/remote_apply/on/off)
- `primary_conninfo`, `primary_slot_name` (quando servidor é réplica)
- `recovery_target_time` / `recovery_target_lsn` / `recovery_target_name` (para PITR)
- `max_logical_replication_workers`, `max_sync_workers_per_subscription`

### 4.7 Query Planner
- `random_page_cost`
- `seq_page_cost`
- `cpu_tuple_cost`, `cpu_index_tuple_cost`, `cpu_operator_cost`
- `effective_cache_size`
- `default_statistics_target`
- `constraint_exclusion`
- `cursor_tuple_fraction`
- `join_collapse_limit`, `from_collapse_limit`
- `geqo` e parâmetros relacionados (`geqo_threshold`, `geqo_effort`, etc. — avançado)
- `jit`, `jit_above_cost`, `jit_optimize_above_cost`, `jit_inline_above_cost`

### 4.8 Paralelismo
- `max_parallel_workers_per_gather`
- `max_parallel_workers`
- `max_parallel_maintenance_workers`
- `parallel_setup_cost`, `parallel_tuple_cost`
- `min_parallel_table_scan_size`, `min_parallel_index_scan_size`
- `force_parallel_mode` (debug)

### 4.9 Autovacuum
- `autovacuum` (on/off)
- `autovacuum_max_workers`
- `autovacuum_naptime`
- `autovacuum_vacuum_threshold`, `autovacuum_vacuum_scale_factor`
- `autovacuum_vacuum_insert_threshold`, `autovacuum_vacuum_insert_scale_factor`
- `autovacuum_analyze_threshold`, `autovacuum_analyze_scale_factor`
- `autovacuum_freeze_max_age`, `autovacuum_multixact_freeze_max_age`
- `autovacuum_vacuum_cost_delay`, `autovacuum_vacuum_cost_limit`
- `vacuum_freeze_min_age`, `vacuum_freeze_table_age`
- `vacuum_multixact_freeze_min_age`, `vacuum_multixact_freeze_table_age`
- `vacuum_cost_delay`, `vacuum_cost_limit`, `vacuum_cost_page_hit`, `vacuum_cost_page_miss`, `vacuum_cost_page_dirty`

### 4.10 Logging (Error Reporting and Logging)
- `log_destination` (stderr/csvlog/jsonlog/syslog)
- `logging_collector`
- `log_directory`, `log_filename`
- `log_rotation_age`, `log_rotation_size`, `log_truncate_on_rotation`
- `log_min_messages`, `log_min_error_statement`
- `log_min_duration_statement` (slow query log — campo destacado no modo Simples: "logar queries mais lentas que ___ ms")
- `log_min_duration_sample`, `log_statement_sample_rate`
- `log_checkpoints`
- `log_connections`, `log_disconnections`
- `log_duration`
- `log_error_verbosity`
- `log_hostname`
- `log_line_prefix`
- `log_lock_waits`
- `log_recovery_conflict_waits`
- `log_statement` (none/ddl/mod/all)
- `log_replication_commands`
- `log_temp_files`
- `log_autovacuum_min_duration`
- `log_parameter_max_length`

### 4.11 Estatísticas em runtime
- `track_activities`, `track_activity_query_size`
- `track_counts`
- `track_io_timing`
- `track_wal_io_timing`
- `track_functions` (none/pl/all)
- `compute_query_id`
- `stats_fetch_consistency`

### 4.12 Configurações padrão do cliente
- `search_path`
- `default_tablespace`, `temp_tablespaces`
- `default_table_access_method`
- `check_function_bodies`
- `default_transaction_isolation`, `default_transaction_read_only`, `default_transaction_deferrable`
- `session_replication_role`
- `statement_timeout`
- `lock_timeout`
- `idle_in_transaction_session_timeout`
- `idle_session_timeout`
- `datestyle`, `intervalstyle`
- `timezone`, `timezone_abbreviations`
- `lc_messages`, `lc_monetary`, `lc_numeric`, `lc_time`
- `default_text_search_config`
- `bytea_output`
- `extra_float_digits`

### 4.13 Gerenciamento de locks
- `deadlock_timeout`
- `max_locks_per_transaction`
- `max_pred_locks_per_transaction`
- `max_pred_locks_per_relation`
- `max_pred_locks_per_page`

### 4.14 Extensões pré-carregadas
- `shared_preload_libraries` (UI dedicada — ver seção 6, evita edição manual de string)
- `session_preload_libraries`, `local_preload_libraries`

### 4.15 Customized options
- Suporte a parâmetros `namespace.parametro` adicionados por extensões (ex.: `pg_stat_statements.max`, `pg_stat_statements.track`, `pgaudit.log`, `auto_explain.log_min_duration`) — a UI deve detectar dinamicamente quais extensões estão ativas e mostrar a seção de configuração específica de cada uma.

### 4.16 Presets e perfis de workload
Botão "Aplicar perfil": **OLTP (transacional)**, **OLAP/Data Warehouse (analítico)**, **Misto**, **Baixa memória (dev)** — cada um ajusta em lote o conjunto de parâmetros recomendado (similar ao PGTune), sempre mostrando diff antes de aplicar.

### 4.17 `pg_hba.conf` — Controle de acesso
Editor visual em formato de tabela com **drag-and-drop para reordenar regras** (ordem importa no Postgres):
- Colunas: Tipo (`local`/`host`/`hostssl`/`hostnossl`/`hostgssenc`/`hostnogssenc`), Banco de dados (all/específico/lista/`sameuser`/`samerole`), Usuário (all/específico/lista), Endereço/CIDR, Método de autenticação (`trust`, `reject`, `scram-sha-256`, `md5`, `password`, `gss`, `sspi`, `ident`, `peer`, `ldap`, `radius`, `cert`), opções extras (ex. `clientcert=verify-full`).
- Validação em tempo real (não deixa salvar regra conflitante/incoerente sem avisar).
- Botão "testar conexão" simulando usuário+IP+banco para saber qual regra vai bater.
- Modo Simples: wizard "Quem pode acessar esse banco?" com opções prontas (só de dentro da rede Docker, IP específico, qualquer lugar com senha, exigir SSL).

### 4.18 `pg_ident.conf`
Mapeamento de usuários de sistema para usuários Postgres (para métodos `ident`/`peer`), editor de tabela simples (map name, system-username, pg-username).

### 4.19 Variáveis de ambiente do container
Edição de env vars adicionais do container Docker (para casos avançados/plugins específicos da imagem).

---

## 5. Gestão de objetos do banco de dados (equivalente completo a um pgAdmin, só que mais bonito)

Navegador em árvore à esquerda: Servidor → Bancos de dados → Schemas → (Tabelas, Views, Views Materializadas, Sequences, Functions/Procedures, Triggers, Extensions, Foreign Tables, Types, Domains, Publications, Subscriptions).

### 5.1 Bancos de dados
- Criar (nome, owner, encoding, collation, template, tablespace, connection limit)
- Editar (renomear, trocar owner, connection limit)
- Excluir (com proteção contra excluir banco com conexões ativas — opção de derrubar conexões)
- Ver tamanho, número de conexões, número de tabelas

### 5.2 Schemas
- Criar/renomear/excluir, definir owner, definir privilégios padrão (`ALTER DEFAULT PRIVILEGES`)

### 5.3 Tabelas
- Criar via formulário visual: nome, colunas (nome, tipo, tamanho/precisão, not null, default, gerada/computed), chave primária, unique constraints, foreign keys (com ação `ON DELETE`/`ON UPDATE`: cascade/restrict/set null/set default/no action), check constraints, tablespace, `WITH (fillfactor=...)`, `UNLOGGED`.
- Editor de estrutura (adicionar/remover/renomear coluna, mudar tipo, mudar default, mudar nullability) — sempre mostrando o SQL equivalente e avisando se a operação faz lock/reescreve a tabela.
- **Particionamento**: criar tabela particionada (RANGE, LIST, HASH), gerenciar partições (criar, anexar, desanexar, converter partição existente), wizard de particionamento automático por data (criar partição mensal automaticamente).
- Visualizar/editar dados como planilha (grid): paginação, ordenação por coluna, filtros por coluna (com operadores por tipo), edição inline de célula, adicionar/excluir linha, copiar/colar de Excel, exportar seleção.
- Importar dados: CSV, JSON, Excel, SQL — com mapeamento de colunas, preview antes de importar, opção upsert.
- Exportar dados: CSV, JSON, Excel, SQL insert, formato `COPY`.
- Ver tamanho da tabela (dados + índices + toast), número de linhas (estimado e exato sob demanda), bloat estimado.

### 5.4 Índices
- Criar via formulário: tipo (`btree`, `hash`, `gin`, `gist`, `spgist`, `brin`), colunas/expressões, `UNIQUE`, `CONCURRENTLY` (padrão em produção, com aviso), parcial (`WHERE`), `INCLUDE` columns, ordenação (`ASC`/`DESC`/`NULLS FIRST/LAST`), operator class, tablespace, fillfactor.
- Listar índices por tabela com: tamanho, uso (scans, tuplas lidas — de `pg_stat_user_indexes`), sugestão de índices não utilizados para remoção, sugestão de índices faltantes (baseado em queries lentas do `pg_stat_statements`/`auto_explain`).
- Rebuild de índice (`REINDEX CONCURRENTLY`).

### 5.5 Views e Materialized Views
- Criar/editar via editor SQL com preview do resultado.
- Materialized views: botão "Refresh" (com/sem `CONCURRENTLY`), agendamento automático de refresh (cron), ver última atualização.

### 5.6 Sequences
- Criar/editar: incremento, min/max, start, cache, cycle, owned by (coluna).
- Ver valor atual, opção de resetar (`setval`/`restart`).

### 5.7 Functions / Procedures
- Editor de código com syntax highlighting para `plpgsql`, `sql`, e outras linguagens instaladas (`plpython3u`, `plperl`, etc., se habilitadas).
- Definir: parâmetros (nome, tipo, `IN`/`OUT`/`INOUT`/`VARIADIC`, default), tipo de retorno (escalar, `TABLE`, `SETOF`, `void`), volatilidade (`IMMUTABLE`/`STABLE`/`VOLATILE`), `SECURITY DEFINER`/`INVOKER`, linguagem, custo estimado, `PARALLEL SAFE/UNSAFE/RESTRICTED`.
- Botão "Executar" com formulário de parâmetros gerado automaticamente, exibindo resultado.
- Diferenciação visual entre Function e Procedure (`CALL`).

### 5.8 Triggers
- Criar via formulário: nome, tabela, timing (`BEFORE`/`AFTER`/`INSTEAD OF`), eventos (`INSERT`/`UPDATE`/`DELETE`/`TRUNCATE`, múltiplos), nível (`ROW`/`STATEMENT`), condição `WHEN`, função a executar (existente ou criar nova inline), ordem de execução quando há múltiplos triggers.
- Habilitar/desabilitar trigger sem excluir.
- **Event Triggers** (nível de banco/DDL): criar, associar a comandos DDL específicos (`CREATE TABLE`, `DROP TABLE`, etc.).

### 5.9 Extensions
Ver seção 6 dedicada.

### 5.10 Types e Domains
- Criar tipos compostos, enums (`CREATE TYPE ... AS ENUM`, com editor de lista de valores e reordenação), tipos range, domains (tipo base + constraints).

### 5.11 Roles / Usuários / Grupos
- Criar role: nome, senha, `LOGIN`/`NOLOGIN`, `SUPERUSER`, `CREATEDB`, `CREATEROLE`, `INHERIT`, `REPLICATION`, `BYPASSRLS`, connection limit, validade da senha (`VALID UNTIL`).
- Gerenciar membership (roles dentro de roles/grupos).
- **Matriz de privilégios visual**: grid banco×schema×tabela×role mostrando SELECT/INSERT/UPDATE/DELETE/TRUNCATE/REFERENCES/TRIGGER com checkboxes, aplicável em lote.
- `ALTER DEFAULT PRIVILEGES` via UI (privilégios que novos objetos herdam automaticamente).
- **Row Level Security (RLS)**: habilitar por tabela, criar policies (nome, comando aplicável, roles afetadas, expressão `USING`/`WITH CHECK`) via formulário assistido.
- Rotação de senha agendada, expiração de senha com aviso.

### 5.12 Tablespaces
- Criar/editar/excluir, associar a caminho no volume, ver uso.

### 5.13 Foreign Data Wrappers / Foreign Tables
- Gerenciar FDWs instaladas (`postgres_fdw`, `file_fdw`, etc.), criar servidores remotos, user mappings, foreign tables via wizard.

### 5.14 Replicação lógica
- Publications: criar (todas as tabelas / tabelas específicas / schema), definir operações replicadas (insert/update/delete/truncate).
- Subscriptions: criar apontando para outro servidor (gerenciado pela plataforma ou externo), status de sincronização, botão refresh/resync, pausar/retomar.

### 5.15 Editor SQL (Query Tool)
- Editor com autocomplete de tabelas/colunas/funções, múltiplas abas, histórico de queries executadas, queries salvas/favoritas, atalhos de teclado (executar, formatar SQL).
- `EXPLAIN` / `EXPLAIN ANALYZE` com **visualização gráfica do plano de execução** (árvore, custos, tempo real por nó, destaque de nós problemáticos — seq scan em tabela grande, etc.).
- Resultado em grid paginado, exportável (CSV/JSON/Excel), com opção de gerar gráfico rápido a partir do resultado.
- Modo somente-leitura opcional (proteção contra DML acidental em produção), confirmação obrigatória para `DROP`/`DELETE`/`TRUNCATE`/`UPDATE` sem `WHERE`.
- Compartilhar query com outro usuário da plataforma (link).

---

## 6. Gestão de extensões

- Tela com todas as extensões disponíveis na imagem (`pg_available_extensions`), com: nome, descrição, versão disponível, versão instalada (se aplicável), categoria (performance, segurança, tipos de dados, geoespacial, full-text search, etc.), se precisa estar em `shared_preload_libraries` (e nesse caso avisar que precisa restart).
- Toggle simples para habilitar/desabilitar (`CREATE EXTENSION` / `DROP EXTENSION`), com resolução de dependências entre extensões.
- Extensões suportadas de fábrica (lista mínima que a imagem customizada deve trazer compilada):
  - `pg_stat_statements` (obrigatória, habilitada por padrão — base do monitoramento de queries)
  - `pgcrypto`
  - `uuid-ossp` / `pgcrypto gen_random_uuid` 
  - `pg_trgm` (busca fuzzy/similaridade)
  - `unaccent`
  - `hstore`
  - `citext`
  - `postgis` (geoespacial)
  - `pg_cron` (jobs agendados dentro do banco)
  - `pgaudit` (auditoria detalhada)
  - `pg_repack` (reorganizar tabelas sem lock longo)
  - `pglogical` (replicação lógica avançada)
  - `timescaledb` (séries temporais) — opcional/imagem alternativa
  - `pg_partman` (particionamento automático)
  - `auto_explain` (log automático de planos de queries lentas)
  - `amcheck` (verificação de corrupção de índices/tabelas)
  - `pg_visibility`, `pgstattuple` (bloat/inspeção física)
  - `tablefunc`, `ltree`, `intarray`
- Cada extensão com config específica ganha sub-tela própria (ex.: `pg_cron` ganha um gerenciador de jobs agendados dentro do banco, com UI de cron; `pgaudit` ganha tela de configuração de quais operações auditar).

---

## 7. Monitoramento do PostgreSQL

### 7.1 Visão de conexões e atividade (`pg_stat_activity`)
- Lista em tempo real de todas as sessões: PID, usuário, banco, aplicação (`application_name`), IP de origem, estado (`active`/`idle`/`idle in transaction`/`idle in transaction (aborted)`), query atual, tempo de execução, tempo desde início da transação, wait event.
- Ações por sessão: **cancelar query** (`pg_cancel_backend`), **encerrar conexão** (`pg_terminate_backend`), ver query completa/plano.
- Filtros: por banco, por usuário, por estado, queries rodando há mais de X segundos (destacar em vermelho).
- Gráfico de conexões ao longo do tempo, por estado, com linha de limite (`max_connections`) e alerta quando perto do limite.

### 7.2 Queries (via `pg_stat_statements`)
- Tabela ordenável: query normalizada, número de chamadas, tempo total, tempo médio, tempo min/max, desvio padrão, linhas retornadas/afetadas, cache hit %, tempo de I/O (leitura/escrita de blocos), tempo de planejamento vs execução.
- Top queries por: tempo total acumulado, tempo médio, número de chamadas, mais I/O.
- Botão "Reset estatísticas".
- Botão "Ver plano de execução" (roda `EXPLAIN` na hora, com aviso se envolver escrita).
- Correlação com `auto_explain` para ver plano real capturado no momento em que rodou lento.

### 7.3 Bancos de dados (`pg_stat_database`)
- Por banco: número de conexões, transações commitadas/revertidas, blocos lidos do disco vs do cache, tuplas retornadas/buscadas/inseridas/atualizadas/deletadas, conflitos, deadlocks, tempo de I/O, tamanho em disco.

### 7.4 Tabelas e Índices (`pg_stat_user_tables`, `pg_stat_user_indexes`, `pg_statio_*`)
- Por tabela: scans sequenciais vs por índice, tuplas lidas/inseridas/atualizadas/deletadas/hot-updated, tuplas mortas estimadas (bloat), última vez que rodou vacuum/autovacuum/analyze/autoanalyze, cache hit ratio.
- Por índice: número de scans, tuplas lidas, tamanho, "índice nunca usado" destacado como candidato a remoção.
- Ranking de tabelas com mais bloat, sugestão de `VACUUM FULL`/`pg_repack`.

### 7.5 Locks e Bloqueios
- Visão de locks ativos (`pg_locks` cruzado com `pg_stat_activity`): quem está bloqueando quem (árvore/grafo de bloqueio), tipo de lock, modo, tabela/objeto afetado.
- Detecção e histórico de deadlocks.
- Alerta em tempo real quando uma query fica esperando lock por mais de X segundos.

### 7.6 Replicação
- Se o servidor é primário: lista de réplicas conectadas (`pg_stat_replication`), lag em bytes e em tempo, estado de sincronização (streaming/catchup), slots de replicação (ativos/inativos, WAL retido por slot — alerta se slot inativo estiver acumulando WAL e enchendo disco).
- Se o servidor é réplica: lag em relação ao primário, último WAL recebido/aplicado, se está em modo `hot_standby`, botão de promoção a primário.

### 7.7 WAL e Checkpoints
- Taxa de geração de WAL (MB/s), gráfico de checkpoints (tempo entre eles, se estão sendo forçados por tamanho ou por tempo — sinal de tuning necessário), buffers de WAL.

### 7.8 Vacuum / Autovacuum
- Jobs de autovacuum rodando agora (tabela, fase, progresso — via `pg_stat_progress_vacuum`), histórico de execuções, tabelas há mais tempo sem vacuum, previsão de wraparound de transaction ID (alerta crítico configurável).

### 7.9 Background Writer / Checkpointer (`pg_stat_bgwriter`)
- Buffers escritos pelo checkpointer vs bgwriter vs backend direto (indicador de tuning de `shared_buffers`/checkpoints).

### 7.10 Progresso de operações longas
- `pg_stat_progress_create_index`, `pg_stat_progress_vacuum`, `pg_stat_progress_cluster`, `pg_stat_progress_basebackup`, `pg_stat_progress_copy` — barra de progresso visual para qualquer operação longa em andamento.

### 7.11 Tamanhos e crescimento
- Tamanho de cada banco, schema, tabela, índice — atual e histórico (gráfico de crescimento), projeção de quando o disco vai encher no ritmo atual.

### 7.12 Health Score
- Um "score de saúde" agregado por servidor (0-100) calculado a partir de: cache hit ratio, conexões próximas do limite, bloat, lag de replicação, falhas de backup, uso de disco — com detalhamento do que está pesando a nota, tipo "nota de crédito" do banco.

---

## 8. Monitoramento do Sistema Operacional / Docker

- **CPU**: uso % por container (limit vs uso real), uso do host.
- **Memória**: uso RAM por container (limit vs uso real, incluindo shared_buffers do Postgres), swap.
- **Disco**: uso do volume de dados (usado/livre/%), IOPS de leitura/escrita, throughput MB/s, latência de disco.
- **Rede**: tráfego de entrada/saída por container (bytes/s, pacotes/s), conexões TCP abertas.
- **Container**: status (running/restarting/exited), uptime, número de restarts, últimos eventos do Docker (OOM killed, restart por healthcheck falho), healthcheck status.
- **Host**: se a plataforma tiver acesso, visão geral do servidor físico/VM (CPU, RAM, disco totais e disponíveis, load average, número de containers rodando).
- Gráficos de série temporal para tudo acima, com retenção configurável (curto prazo em alta resolução, longo prazo agregado — tipo Prometheus).
- Alertas configuráveis por limite (CPU > X% por Y minutos, disco > X%, memória > X%, container reiniciou N vezes em Y minutos).

---

## 9. Logs

### 9.1 Logs do PostgreSQL
- Visualizador em tempo real (tail -f) com auto-scroll, e histórico pesquisável.
- Parsing estruturado (usando `log_destination = csvlog` ou `jsonlog`) para permitir filtros por: nível (LOG/WARNING/ERROR/FATAL/PANIC), banco, usuário, aplicação, PID, intervalo de tempo, texto livre (busca), duração da query (quando aplicável).
- Destaque colorido por severidade.
- Correlação: clicar num log de erro e ver a query completa relacionada (quando disponível).
- Download de logs (período selecionado), rotação/retenção configurável.
- Alerta configurável por padrão de log (ex.: notificar sempre que aparecer `FATAL` ou `PANIC`, ou uma regex customizada).

### 9.2 Logs do container Docker
- stdout/stderr do container, separados dos logs internos do Postgres.

### 9.3 Logs de auditoria da plataforma
- Toda ação de usuário na plataforma (criar servidor, mudar config, excluir tabela, restaurar backup, alterar permissão) registrada com: quem, quando, IP, ação, diff antes/depois quando aplicável. Pesquisável e exportável (requisito de compliance).

---

## 10. Backup

### 10.1 Backup manual (sob demanda)
- Botão "Fazer backup agora" em qualquer servidor.
- Escolha de método: `pg_dump` (formato custom/plain/directory/tar), `pg_dumpall` (cluster completo, incluindo roles/tablespaces), `pg_basebackup` (backup físico, base para PITR).
- Escopo: cluster inteiro, banco específico, schema específico, tabelas específicas.
- Opções: compressão (nível), paralelismo (`-j`), incluir/excluir dados de tabelas específicas (schema-only para algumas), incluir blobs.

### 10.2 Rotinas de backup agendadas
- Wizard de agendamento: frequência (diário, semanal, mensal, cron customizado via UI amigável tipo "todo dia às 03:00"), tipo de backup (lógico/físico), escopo, política de retenção (manter últimos N backups, manter por X dias, política tipo "avô-pai-filho" — diário 7 dias, semanal 4 semanas, mensal 12 meses).
- Múltiplas rotinas por servidor (ex.: backup lógico diário + WAL archiving contínuo para PITR).
- Notificação de sucesso/falha (email, Slack, Discord, Telegram, webhook genérico).
- Verificação automática de integridade pós-backup (opcional: restaurar em ambiente isolado de teste e validar).

### 10.3 Continuous Archiving / PITR
- Configuração de `archive_command`/WAL archiving contínuo apontando para o storage escolhido, permitindo restauração para **qualquer ponto no tempo** dentro da janela de retenção, não só para o momento do último backup.
- Linha do tempo visual mostrando pontos de backup completo + WALs disponíveis, permitindo escolher visualmente "restaurar para este exato momento".

### 10.4 Destinos de armazenamento (multi-storage)
- **Local** (disco do host/volume dedicado a backups)
- **Google Drive** — fluxo OAuth2 (o usuário clica "Conectar Google Drive", autoriza via tela do Google, token armazenado de forma criptografada), escolha de pasta de destino, verificação de espaço disponível.
- **Amazon S3 / compatível com S3** (Backblaze B2, MinIO, Wasabi, DigitalOcean Spaces, etc.) — access key, secret key, bucket, região, endpoint customizado.
- **Azure Blob Storage**
- **Dropbox** (OAuth2, similar ao Google Drive)
- **FTP/SFTP** genérico
- **WebDAV** genérico
- Possibilidade de configurar **múltiplos destinos simultâneos** por rotina (ex.: local + Google Drive, para redundância 3-2-1).
- Criptografia dos backups em repouso (chave gerenciada pela plataforma ou fornecida pelo usuário), com opção de criptografia client-side antes do upload (o storage remoto nunca vê os dados em claro).

### 10.5 Gestão de backups existentes
- Lista de todos os backups (por servidor): data/hora, tipo, tamanho, destino(s), status (sucesso/falha/em andamento), duração, quem/o quê disparou (manual/agendado).
- Download manual de um backup específico.
- Exclusão manual (respeitando ou sobrepondo a política de retenção, com confirmação).
- Log detalhado de cada execução de backup (para debug de falhas).

### 10.6 Restore
- Wizard de restauração: escolher backup (ou ponto no tempo para PITR) → escolher destino (sobrescrever servidor original / criar novo servidor / restaurar em servidor existente diferente) → escolher escopo (tudo / banco específico / schema / tabelas específicas via `pg_restore --table`) → preview do que vai ser restaurado → confirmação com aviso claro de impacto → execução com barra de progresso.
- Restauração de tabela única sem afetar o resto do banco (quando o backup for em formato `custom`/`directory`, via `pg_restore -t`).
- "Restaurar como novo servidor" — útil para criar cópia de produção em homologação, ou investigar um problema sem tocar no servidor original.
- Teste de restauração automática agendada (validação periódica de que os backups realmente restauram, em ambiente descartável).

---

## 11. Alertas e Notificações

- Central de alertas configuráveis por servidor ou globalmente: métrica, operador (>, <, =), limite, janela de tempo (ex. "por mais de 5 minutos"), severidade.
- Catálogo de alertas prontos: CPU alta, RAM alta, disco cheio/quase cheio, muitas conexões, conexão perto do `max_connections`, réplica com lag alto, slot de replicação inativo acumulando WAL, backup falhou, backup não roda há mais de X dias, certificado SSL expirando, query rodando há mais de X minutos, deadlock detectado, autovacuum atrasado, transaction ID wraparound se aproximando, container reiniciou inesperadamente, disco de backup com pouco espaço.
- Canais de notificação: e-mail, Slack, Discord, Telegram, webhook genérico (JSON customizável), SMS (opcional/integração terceira), push notification no navegador.
- Escalonamento (se não reconhecido em X minutos, notificar outro canal/pessoa) — avançado.
- Silenciar/pausar alertas temporariamente (manutenção programada).

---

## 12. Segurança e Compliance

- Gestão de certificados SSL/TLS por servidor: upload manual, ou emissão automática (Let's Encrypt/ACME se houver domínio configurado), renovação automática, alerta de expiração.
- Política de senha (para roles do Postgres e para usuários da plataforma): tamanho mínimo, complexidade, expiração, histórico (não repetir últimas N).
- **RBAC da plataforma** (diferente do RBAC do Postgres): Organizações/Times → Usuários → Papéis (Admin, Operador, Desenvolvedor, Visualizador/Somente leitura) → Permissões granulares por servidor ou grupo de servidores (quem pode ver, quem pode editar config, quem pode fazer restore, quem pode excluir servidor).
- Autenticação: login/senha, 2FA (TOTP), SSO opcional (OAuth2/OIDC/SAML — Google, Microsoft, GitHub), API keys para automação/CI-CD.
- Segredos (senhas de banco, chaves de storage) armazenados em vault criptografado, nunca em texto plano, nunca expostos em logs.
- Acesso ao Docker via `docker-socket-proxy` restrito às operações necessárias (nunca socket direto).
- Registro de auditoria completo (ver 9.3), exportável para SIEM externo.
- Suporte a `pgaudit` para auditoria fina de DML/DDL dentro do próprio Postgres.
- Mascaramento de dados sensíveis na visualização de tabelas (opcional, por coluna marcada como sensível — CPF, cartão, etc.).

---

## 13. Alta disponibilidade e Replicação (avançado)

- Criar réplica de leitura (streaming replication) com um clique a partir de um servidor existente.
- Promover réplica a primário (manual).
- Failover automático opcional via Patroni (para quem quer HA de verdade) — feature avançada, configurável, com etcd/Consul como DCS.
- Connection pooling gerenciado: deploy de PgBouncer ou PgCat na frente do servidor, com configuração de modo (session/transaction/statement), tamanho do pool, via UI.
- Load balancing de leitura entre réplicas (opcional, via proxy).

---

## 14. Migração e Importação

- Wizard de migração de banco externo (Postgres já existente fora da plataforma) para dentro de um servidor gerenciado — via `pg_dump`/`pg_restore` ou conexão direta com barra de progresso.
- Import de dump SQL de outros sistemas (na medida do possível — MySQL via ferramenta de conversão, por exemplo, como funcionalidade estendida).

---

## 15. API, Automação e Extensibilidade

- API REST completa cobrindo 100% das ações da UI (criar servidor, alterar config, disparar backup, consultar métricas, etc.), documentada (OpenAPI/Swagger).
- Webhooks de eventos (servidor criado, backup concluído, alerta disparado, etc.) para integração externa.
- CLI companion (`gestpg` ou nome do produto) para uso via terminal/scripts/CI-CD.
- Terraform provider (avançado/futuro) para provisionar servidores como infraestrutura como código.
- Suporte a "Infrastructure as Code": exportar configuração de um servidor como YAML/JSON versionável, e aplicar esse arquivo para recriar/atualizar configuração (GitOps-friendly).

---

## 16. UI/UX — Requisitos gerais

- **Dois modos** (Simples/Avançado) com toggle persistente por usuário, mas por tela — um usuário avançado pode preferir simples só na tela de criação de servidor, por exemplo.
- Tooltips e textos de ajuda em **português claro** (não jargão) em todo campo do modo Simples, com link "saiba mais" para documentação técnica.
- Onboarding guiado na primeira vez (tour pelas telas principais).
- Busca global (command palette, `Ctrl+K`) para pular direto para qualquer servidor, tabela, configuração.
- Tema claro/escuro, responsivo (funciona em tablet, uso ocasional em celular para checar alertas).
- Confirmações claras e específicas para ações destrutivas (nunca um "OK" genérico — sempre dizer exatamente o que vai acontecer, e para ações críticas exigir digitar o nome do objeto).
- Feedback visual imediato de toda ação (loading states, toasts de sucesso/erro, barras de progresso para operações longas).
- Acessibilidade (contraste adequado, navegação por teclado, leitor de tela nos fluxos principais).
- Internacionalização: português (padrão) e inglês no mínimo, estrutura pronta para outros idiomas.
- Exportação de qualquer gráfico/relatório do dashboard como imagem ou PDF.

---

## 17. Requisitos não-funcionais

- **Instalação**: um único `docker-compose up` (ou script de instalação) sobe a plataforma inteira (backend, frontend, banco de metadados, coletor de métricas, proxy do Docker socket).
- **Banco de metadados próprio**: Postgres separado guardando servidores registrados, usuários/roles da plataforma, histórico de configuração (versionado, com diff), auditoria, agendamentos de backup, alertas configurados, credenciais de storage (criptografadas).
- **Backup do próprio metadado da plataforma** (não pode faltar backup de quem cuida de backup).
- **Escalabilidade**: uma instância da plataforma deve gerenciar dezenas/centenas de servidores Postgres sem degradar.
- **Performance**: dashboard principal carrega em menos de 2s com até 50 servidores monitorados; coleta de métricas não deve impactar performance dos servidores monitorados (polling eficiente, batch, cache).
- **Retenção de métricas**: alta resolução (ex. 10s) por curto prazo (24-48h), agregação (1min/5min/1h) para médio e longo prazo (dias/meses), configurável.
- **Multi-tenancy**: suporte a múltiplas organizações isoladas na mesma instalação da plataforma (opcional, para uso SaaS futuro).
- **Observabilidade da própria plataforma**: métricas e logs do backend/frontend expostos (Prometheus-compatible), health checks.
- **Resiliência**: se o coletor de métricas cair, não deve afetar os servidores Postgres gerenciados (desacoplamento total — a plataforma monitora, não é dependência de runtime dos bancos).
- **Atualização da plataforma**: processo de upgrade documentado e sem perda de dados/configuração, migrations versionadas do banco de metadados.
- **Documentação**: manual do usuário (modo Simples) e referência técnica completa (modo Avançado) publicadas junto com o produto.

---

## 18. Fora de escopo (por ora, mas registrado para roadmap)

- Suporte a outros bancos (MySQL, MongoDB, etc.) — pode virar produto-irmão no futuro com a mesma UI.
- Marketplace de templates de aplicação (tipo Coolify/Railway) — este produto foca em ser especialista em Postgres, não um PaaS genérico.
- Billing/cobrança (só relevante se virar SaaS multi-tenant comercial).
