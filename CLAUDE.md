# CLAUDE.md — gest-postgres

Contexto do projeto pra qualquer sessão futura. Ler antes de trabalhar.

## O que é

Plataforma de gestão de instâncias PostgreSQL em Docker: provisionar servidor via UI, configurar tudo do Postgres visualmente, gerenciar extensões, monitorar métricas (Postgres + SO), backup/restore. Público: iniciante (modo Simples, presets) e DBA (modo Avançado, 100% dos parâmetros).

- Ideia original / justificativa de stack: [IDEIA.md](./IDEIA.md)
- Lista completa de requisitos (produto final, todas as "perfumarias"): [REQUISITOS.md](./REQUISITOS.md)

**Fase atual: MVP.** Foco só na seção "Escopo MVP" abaixo. Não implementar itens de "Backlog pós-MVP" sem pedido explícito do usuário, mesmo que estejam documentados em REQUISITOS.md.

## Stack decidida

- **Backend**: Go. `docker/docker/client` (Docker Engine API), `pgx` (driver Postgres), goroutines pra polling de métricas.
- **Frontend**: React + Next.js + shadcn/ui + TanStack Query + Recharts/Tremor pros gráficos.
- **Tempo real**: WebSocket direto do backend Go (`gorilla/websocket` ou `nhooyr.io/websocket`), sem reinventar no front.
- **Banco de metadados**: Postgres separado (servidores registrados, users da plataforma, histórico de config, auditoria, agendamento de backup) — não confundir com os Postgres gerenciados.
- **Docker socket**: nunca acesso direto. Usar `docker-socket-proxy` (`tecnativa/docker-socket-proxy`) liberando só create/start/stop/inspect.

Ainda não decidido / definir quando chegar lá: estrutura de pastas, ORM/query builder no Go (ou SQL puro com `pgx`), lib de auth, formato de deploy final (compose vs binário + compose gerado).

## Convenção de tracking de requisitos

Todo requisito do MVP abaixo é um checkbox. Ao implementar:
1. Marca `[x]` e risca o item: `- [x] ~~texto~~`.
2. Se implementou parcial, deixa `[ ]` e anota o que falta entre parênteses.
3. Não risca por "escrevi o design" — só quando funciona de ponta a ponta (build/roda).

---

## Escopo MVP

### Servidores (ciclo de vida básico)
- [x] ~~Criar servidor via Docker API: nome, versão do Postgres, usuário/senha, porta, preset de recursos (Pequeno/Médio/Grande)~~
- [x] ~~Volume nomeado por instância + rede Docker isolada~~
- [ ] Listar servidores com status (rodando/parado/erro), versão (falta: conexões atuais — sem monitoramento ainda)
- [x] ~~Start / Stop / Restart~~
- [ ] Editar servidor (nome, recursos, porta)
- [x] ~~Excluir servidor (confirmação, opção manter/apagar volume)~~

Verificado ponta a ponta em droplet Debian real (wipe total → clone limpo → `sudo ./setup.sh` → criar/start/stop/restart/excluir servidor pela UI, Postgres realmente aceitando conexão). Ver histórico de commits a partir de `f5a557d` até `ee97d0e`.

**Além do MVP**: auto-descoberta de Postgres rodando em containers Docker (fora do MVP original, mas dentro do que a arquitetura atual consegue — sem acesso ao host além da API Docker, então Postgres nativo fora de container fica fora de alcance). Botão "Procurar servidores" na tela inicial lista containers que parecem Postgres e ainda não estão cadastrados; "Cadastrar" pede credenciais reais e só salva depois de confirmar conexão de verdade (`docker network connect` na rede gerenciada se precisar, depois um `SELECT` real) — nada fica registrado com senha errada. Não cria container/volume novo, só passa a gerenciar o que já existe.

### Configuração do Postgres (subset essencial, não tudo de REQUISITOS.md §4)
- [x] ~~Form com os parâmetros mais impactantes: `max_connections`, `shared_buffers`, `work_mem`, `maintenance_work_mem`, `effective_cache_size`, `log_min_duration_statement`~~
- [x] ~~Presets calculam esses valores automaticamente a partir do preset de recursos~~
- [x] ~~Aplicar mudança → reload ou avisa que precisa restart~~
- [ ] `pg_hba.conf` básico: tabela simples de regras (tipo, database, user, CIDR, método), sem drag-and-drop ainda

Achado e corrigido um bug de arquitetura sério aqui: a config inicial entrava como flag `-c` no comando do container, que tem prioridade MAIOR que `ALTER SYSTEM` — nenhuma edição pós-criação nunca ia pegar, nem com restart. Agora tudo (inicial e edições) passa por `ALTER SYSTEM` + reload/restart, mesmo caminho.

Configuração expandida bem além do subset original: ~86 parâmetros geridos, agrupados por categoria (memória, conexões, WAL, autovacuum, logging, etc.), com busca, indicação de quais precisam restart vs. reload, e edição só dos campos alterados. Fora do editável de propósito: `listen_addresses`/`port`/`unix_socket_directories`/certificados/`shared_preload_libraries`/`recovery_target_*`/`restore_command` e toggles de debug (`enable_*`) — mudar isso quebra ou exige orquestração que a plataforma ainda não faz.

### Banco de dados / objetos (mínimo pra ser usável)
- [x] ~~Criar/listar/excluir database~~ (aba Monitoramento, card "Bancos de dados" — excluir usa `DROP DATABASE ... WITH (FORCE)`, Postgres 13+, derruba conexões abertas em vez de falhar; banco principal do servidor não pode ser excluído)
- [x] ~~Criar/listar/excluir tabela via formulário~~ (excluir: botão aparece ao passar o mouse na lista de tabelas)
- [x] ~~Editor SQL básico (rodar query, ver resultado em grid, sem autocomplete ainda)~~ — ganhou syntax highlighting (CodeMirror) e histórico de queries também, além do MVP original
- [x] ~~Ver dados da tabela em grid com paginação~~

### Extensões
- [x] ~~Listar `pg_available_extensions`~~
- [x] ~~Habilitar/desabilitar: `pg_stat_statements`, `uuid-ossp`, `pgcrypto`, `pg_trgm`~~ (e qualquer outra da lista, não só essas 4)

`postgres:X` oficial não vem com `pgvector` nem `pg_cron` compilados — servidores novos agora sobem em cima de `gestpg-postgres:X` (`postgres-image/Dockerfile`), a mesma imagem oficial + esses dois pacotes via apt (repo PGDG que a própria imagem já tem configurado). Buildada localmente pelo `setup.sh` uma vez por versão suportada (13-17) — o backend só faz pull/inspect na criação de servidor, nunca build (permissão do docker-socket-proxy é só isso de propósito). `pgvector` funciona na hora (`CREATE EXTENSION vector`, sem restart). `pg_cron` precisa de `shared_preload_libraries` + `cron.database_name`, mesmo tratamento que já existia pra `pg_stat_statements` — clique em "Habilitar" na aba Extensões cuida disso sozinho (reinicia o container, demora mais, badge "requer restart" avisa antes).

Achado e corrigido um bug sério do próprio Postgres nesse processo: `ALTER SYSTEM SET shared_preload_libraries = 'lib1,lib2'` (uma string só com vírgula dentro) faz o Postgres persistir errado no `postgresql.auto.conf` — grava `= '"lib1,lib2"'` com aspas duplas extras envolvendo tudo, e na subida seguinte ele tenta abrir UM arquivo de lib chamado literalmente "lib1,lib2" e trava em crash loop pra sempre. A sintaxe correta é multi-valor (`ALTER SYSTEM SET shared_preload_libraries = lib1, lib2` — identificadores soltos, sem string por fora). Servidores já existentes com só 1 lib preload nunca bateram nesse bug (só aparece com 2+).

Servidores criados antes dessa mudança continuam na imagem `postgres:X` antiga — sem pgvector/pg_cron disponíveis até serem recriados. Sem migração automática no MVP.

### Monitoramento
- [x] ~~`pg_stat_activity` ao vivo (sessões, query atual, estado), botão cancelar/terminar sessão~~
- [x] ~~Dashboard com gráfico de conexões ao longo do tempo~~ (+ gráfico de CPU/memória — histórico em memória, reseta se o backend reiniciar)
- [x] ~~Top queries lentas via `pg_stat_statements`~~ (aba "Desempenho" — ordenação por tempo total/médio/chamadas, reset de stats, fluxo guiado de habilitação quando a extensão não tá coletando ainda)
- [ ] CPU/RAM/disco por container (docker stats) (falta disco — CPU/RAM ok)

### Logs
- [x] ~~Visualizador de log do Postgres (tail básico, sem parsing estruturado ainda)~~

Tudo isso vive em `/servers/{id}` (clica no nome do servidor na lista) — abas: Monitoramento, Logs, Editor SQL, Tabelas, Extensões, Configuração, Usuários, Desempenho, Objetos, Funções. Backend conecta direto no Postgres gerenciado pela rede `gestpg-managed` (nome do container, não host_port). Verificado ponta a ponta no mesmo droplet a cada feature.

**Além do MVP original**, também saiu nessa leva (pedido explícito do usuário, fora da lista original mas dentro do espírito "gerenciar o banco"):
- Connection string com senha revelável (copiar pra conectar de fora — psql, DBeaver, etc)
- Criar tabela via formulário visual (nome, colunas, tipos, PK, not null, default)
- Gerenciar triggers por tabela (criar/habilitar/desabilitar/excluir)
- Usuários/roles: criar/excluir role, flags (login/superuser/createdb/createrole), matriz de permissões (GRANT/REVOKE SELECT/INSERT/UPDATE/DELETE por tabela)
- Aba "Objetos": Views (criar/listar/excluir), Materialized Views (criar/listar/refresh/excluir), Sequences (criar/listar/excluir), Types/Domains (enum e domain com CHECK, criar/listar/excluir)
- Aba "Funções": functions e procedures — listar (com definição expansível via `pg_get_functiondef`), criar via SQL cru num editor CodeMirror, excluir (suporta overload via assinatura completa)
- Aba "Desempenho": queries lentas via `pg_stat_statements`, com auto-preload da extensão em `shared_preload_libraries` na criação de servidores novos (senão a extensão fica instalada mas não coleta nada) e fluxo guiado de habilitação pra servidores já existentes
- Monitoramento ganhou: lista de databases com tamanho, gráfico de conexões, CPU e memória ao longo do tempo (histórico em memória via goroutine de coleta a cada 15s, reseta se o backend reiniciar)
- Configuração expandida de ~6 pra ~86 parâmetros geridos (ver detalhe acima)

Todos os itens acima testados via curl direto no droplet (criar/listar/refresh/excluir de cada tipo de objeto) e limpos depois. Ver histórico de commits recentes pro detalhe de cada leva.

### Backup / Restore
- [ ] Backup manual sob demanda (`pg_dump`, formato custom)
- [ ] Rotina agendada simples (cron básico: diário/semanal, horário)
- [ ] Storage local only (sem Google Drive/S3 no MVP)
- [ ] Restore de backup (sobrescrever servidor original ou criar novo)
- [ ] Retenção simples: manter últimos N backups

### Plataforma
- [ ] Login/senha (1 usuário admin, sem RBAC multi-nível ainda)
- [x] ~~Dashboard principal com cards de resumo + gráficos do item Monitoramento~~ — CPU/memória/disco/rede agregados de TODOS os containers Docker do host (não só os gerenciados — dá pra ver o que a própria plataforma consome também), + lista por container com destaque pros gerenciados. "Recursos do host" de verdade (fora do mundo Docker) não é possível com a arquitetura atual (backend só fala com o host via Docker socket-proxy, sem acesso a /proc) — disco vem do `/system/df` do Docker (imagens+containers+volumes, exigiu habilitar a categoria `SYSTEM` no proxy), rede é acumulado desde que cada container subiu (não taxa em tempo real)

---

## Backlog pós-MVP ("perfumarias")

Não implementar agora. Detalhe completo em REQUISITOS.md. Resumo do que fica pra depois:

- Multi-storage de backup (Google Drive, S3, Dropbox, FTP), backup físico incremental com PITR real via pgBackRest/Barman (não só `pg_dump` lógico), criptografia de backup, teste automático de restore agendado (restaurar periodicamente pra validar que o backup presta)
- Todos os ~150 parâmetros do `postgresql.conf` (hoje só o subset essencial), editor de arquivo puro, perfis de workload (OLTP/OLAP), `pg_ident.conf`
- `pg_hba.conf` com drag-and-drop e simulador de regra
- Particionamento, RLS/policies, event triggers, FDW, replicação lógica (publications/subscriptions), tipos customizados/domains, tablespaces
- Índices: sugestão de faltantes/não usados (baseado em `pg_stat_statements`), rebuild concorrente
- EXPLAIN visual gráfico (plano de execução legível, não texto cru), autocomplete no editor SQL, queries salvas/compartilhadas
- Monitoramento avançado: locks/deadlock graph, replicação (réplicas/lag/slots), vacuum progress, detecção de bloat (tabelas/índices inchados por vacuum atrasado, com sugestão de ação), alerta de wraparound (`age(datfrozenxid)`), health score, correlação de log com métricas (ver log do Postgres no mesmo lugar que o gráfico de CPU/conexões daquele horário), previsão de capacidade (tendência de crescimento de disco)
- Tuning assistido de autovacuum e memória (sugerir `shared_buffers`/`work_mem`/etc. baseado no hardware real do container)
- Alertas configuráveis multi-canal (email/Slack/Discord/Telegram/webhook): conexões perto do limite, réplica atrasando, disco enchendo, queries travadas, deadlocks
- RBAC multi-usuário/times (granular por servidor: ver / editar / só monitorar), 2FA, SSO, API keys, auditoria completa da plataforma (quem mudou qual config, quando, com opção de reverter — pgAudit no lado do banco + log próprio da plataforma), rotação de credenciais/secrets (senha de superuser não deveria ficar estática pra sempre)
- Certificados TLS geridos automaticamente (emissão e renovação) pras conexões Postgres
- HA (Patroni) com failover automático (promoção de réplica se a primária cair), connection pooling gerenciado (PgBouncer/PgCat), read replicas via UI com roteamento assimétrico (escrita → primária, leitura → réplicas, não round-robin)
- API REST pública documentada, CLI, Terraform provider, IaC export
- Multi-tenancy (organizações)
- Auto-descoberta de Postgres já existentes na máquina (não criados pelo sistema) — ao instalar, varrer a máquina local por instalações Postgres e containers de banco já rodando, sugerir cadastro, pedir senha de cada um se necessário. Sempre local — sem gestão remota por enquanto (possível "cloud" futuro que agregue múltiplas máquinas monitoradas fica pra depois)
- Upgrade de versão maior via wizard (`pg_upgrade`), clonagem rápida de banco (copy-on-write, ambiente de teste idêntico à produção em segundos — tipo Neon), mascaramento de dados (anonimizar dados sensíveis ao clonar produção pra dev/staging), retenção e arquivamento (política automática de quando arquivar/deletar dados antigos)
- Extensões avançadas com UI dedicada (habilitar/desabilitar genérico já cobre instalar; UI dedicada seria pra gerenciar o conteúdo — jobs do pg_cron, políticas do pgaudit, etc): `pgaudit`, `timescaledb`, `pg_partman`, `postgis` (`pgvector` e `pg_cron` já saíram — ver seção Extensões acima)

## Notas pro Claude

- Sempre que fechar um item do MVP, volta nesse arquivo e risca.
- Se usuário pedir algo do backlog antes do MVP fechar, implementa mas pergunta antes se é intencional priorizar fora de ordem.
- Não criar abstração/config pra funcionalidade do backlog "pra facilitar depois" — YAGNI, o MVP é pra sair rápido.
