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

### Configuração do Postgres (subset essencial, não tudo de REQUISITOS.md §4)
- [x] ~~Form com os parâmetros mais impactantes: `max_connections`, `shared_buffers`, `work_mem`, `maintenance_work_mem`, `effective_cache_size`, `log_min_duration_statement`~~
- [x] ~~Presets calculam esses valores automaticamente a partir do preset de recursos~~
- [x] ~~Aplicar mudança → reload ou avisa que precisa restart~~
- [ ] `pg_hba.conf` básico: tabela simples de regras (tipo, database, user, CIDR, método), sem drag-and-drop ainda

Achado e corrigido um bug de arquitetura sério aqui: a config inicial entrava como flag `-c` no comando do container, que tem prioridade MAIOR que `ALTER SYSTEM` — nenhuma edição pós-criação nunca ia pegar, nem com restart. Agora tudo (inicial e edições) passa por `ALTER SYSTEM` + reload/restart, mesmo caminho.

### Banco de dados / objetos (mínimo pra ser usável)
- [ ] Criar/listar/excluir database (dá pra fazer via editor SQL, mas sem UI dedicada ainda)
- [ ] Criar/listar/excluir tabela via formulário (criar: ok via formulário; excluir tabela ainda só via editor SQL)
- [x] ~~Editor SQL básico (rodar query, ver resultado em grid, sem autocomplete ainda)~~ — ganhou syntax highlighting (CodeMirror) e histórico de queries também, além do MVP original
- [x] ~~Ver dados da tabela em grid com paginação~~

### Extensões
- [x] ~~Listar `pg_available_extensions`~~
- [x] ~~Habilitar/desabilitar: `pg_stat_statements`, `uuid-ossp`, `pgcrypto`, `pg_trgm`~~ (e qualquer outra da lista, não só essas 4)

### Monitoramento
- [x] ~~`pg_stat_activity` ao vivo (sessões, query atual, estado), botão cancelar/terminar sessão~~
- [x] ~~Dashboard com gráfico de conexões ao longo do tempo~~ (+ gráfico de CPU/memória — histórico em memória, reseta se o backend reiniciar)
- [ ] Top queries lentas via `pg_stat_statements`
- [ ] CPU/RAM/disco por container (docker stats) (falta disco — CPU/RAM ok)

### Logs
- [x] ~~Visualizador de log do Postgres (tail básico, sem parsing estruturado ainda)~~

Tudo isso vive em `/servers/{id}` (clica no nome do servidor na lista) — abas: Monitoramento, Logs, Editor SQL, Tabelas, Extensões, Configuração, Usuários. Backend conecta direto no Postgres gerenciado pela rede `gestpg-managed` (nome do container, não host_port). Verificado ponta a ponta no mesmo droplet a cada feature. Ver commits `53505d6` até `f59e036`.

**Além do MVP original**, também saiu nessa leva (pedido explícito do usuário, fora da lista original mas dentro do espírito "gerenciar o banco"):
- Connection string com senha revelável (copiar pra conectar de fora — psql, DBeaver, etc)
- Criar tabela via formulário visual (nome, colunas, tipos, PK, not null, default)
- Gerenciar triggers por tabela (criar/habilitar/desabilitar/excluir)
- Usuários/roles: criar/excluir role, flags (login/superuser/createdb/createrole), matriz de permissões (GRANT/REVOKE SELECT/INSERT/UPDATE/DELETE por tabela)

### Backup / Restore
- [ ] Backup manual sob demanda (`pg_dump`, formato custom)
- [ ] Rotina agendada simples (cron básico: diário/semanal, horário)
- [ ] Storage local only (sem Google Drive/S3 no MVP)
- [ ] Restore de backup (sobrescrever servidor original ou criar novo)
- [ ] Retenção simples: manter últimos N backups

### Plataforma
- [ ] Login/senha (1 usuário admin, sem RBAC multi-nível ainda)
- [ ] Dashboard principal com cards de resumo + gráficos do item Monitoramento

---

## Backlog pós-MVP ("perfumarias")

Não implementar agora. Detalhe completo em REQUISITOS.md. Resumo do que fica pra depois:

- Multi-storage de backup (Google Drive, S3, Dropbox, FTP), PITR/WAL archiving contínuo, criptografia de backup, teste automático de restore
- Todos os ~150 parâmetros do `postgresql.conf` (hoje só o subset essencial), editor de arquivo puro, perfis de workload (OLTP/OLAP), `pg_ident.conf`
- `pg_hba.conf` com drag-and-drop e simulador de regra
- Particionamento, RLS/policies, event triggers, FDW, replicação lógica (publications/subscriptions), tipos customizados/domains, tablespaces
- Índices: sugestão de faltantes/não usados, rebuild concorrente
- EXPLAIN visual gráfico, autocomplete no editor SQL, queries salvas/compartilhadas
- Monitoramento avançado: locks/deadlock graph, replicação (réplicas/lag/slots), vacuum progress, bloat, health score
- Alertas configuráveis multi-canal (email/Slack/Discord/Telegram/webhook)
- RBAC multi-usuário/times, 2FA, SSO, API keys, auditoria completa da plataforma
- HA (Patroni), connection pooling gerenciado (PgBouncer/PgCat), read replicas via UI
- API REST pública documentada, CLI, Terraform provider, IaC export
- Multi-tenancy (organizações)
- Upgrade de versão maior via wizard (`pg_upgrade`), clonar servidor
- Extensões avançadas com UI dedicada: `pg_cron` (job scheduler), `pgaudit`, `timescaledb`, `pg_partman`, `postgis`

## Notas pro Claude

- Sempre que fechar um item do MVP, volta nesse arquivo e risca.
- Se usuário pedir algo do backlog antes do MVP fechar, implementa mas pergunta antes se é intencional priorizar fora de ordem.
- Não criar abstração/config pra funcionalidade do backlog "pra facilitar depois" — YAGNI, o MVP é pra sair rápido.
