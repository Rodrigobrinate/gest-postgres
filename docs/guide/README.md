# gest-postgres

Plataforma self-hosted pra provisionar, configurar e monitorar instâncias PostgreSQL rodando em Docker — tudo por uma UI web, sem editar `postgresql.conf` na mão nem decorar comando de `pg_dump`.

Pensado pra dois públicos ao mesmo tempo:

- **Iniciante** — modo Simples, presets de recursos (Pequeno/Médio/Grande) calculam os parâmetros mais impactantes do Postgres sozinhos.
- **DBA** — modo Avançado, ~86 parâmetros do `postgresql.conf` editáveis diretamente, agrupados por categoria.

Ao longo do desenvolvimento o escopo cresceu de "gerenciar Postgres" pra "gerenciar Docker de forma genérica" (containers, redes, volumes, deploy via compose/Dockerfile/Git, reverse proxy Traefik, firewall do host) — ver [Gestão genérica de Docker](docker-infra.md).

> Documentação de uso da plataforma. Para o histórico de decisões de arquitetura, requisitos originais e o que ainda falta (backlog pós-MVP), ver [`CLAUDE.md`](https://github.com/Rodrigobrinate/gest-postgres/blob/main/CLAUDE.md) e [`REQUISITOS.md`](https://github.com/Rodrigobrinate/gest-postgres/blob/main/REQUISITOS.md) no repositório.

## Por onde começar

1. [Instalação](instalacao.md) — um comando (`setup.sh`) num Debian/Ubuntu com root.
2. [Arquitetura](arquitetura.md) — como as peças conversam (Docker Engine API, proxy de socket, banco de metadados).
3. [Servidores](servidores.md) — criar seu primeiro Postgres gerenciado.

## Mapa do sistema

| Área | O que faz |
|---|---|
| [Servidores](servidores.md) | Provisionar, listar, editar, start/stop/restart, excluir, auto-descobrir Postgres já rodando |
| [Configuração](configuracao.md) | ~86 parâmetros do `postgresql.conf` por categoria, `pg_hba.conf` |
| [Banco de dados e objetos](banco-de-dados.md) | Databases, tabelas, editor SQL, views, sequences, types, functions, usuários/permissões |
| [Extensões](extensoes.md) | `pg_stat_statements`, `pgvector`, `pg_cron`, `uuid-ossp`, `pgcrypto`, `pg_trgm` e demais |
| [Connection pooling](pooling.md) | PgBouncer companheiro por servidor |
| [Monitoramento](monitoramento.md) | `pg_stat_activity` ao vivo, CPU/memória/conexões/disco, top queries lentas |
| [Logs](logs.md) | Log do Postgres parseado e filtrável |
| [Backup e restore](backup.md) | `pg_dump` manual/agendado, local ou Google Drive, restore com dois modos |
| [Docker genérico](docker-infra.md) | Containers/redes/volumes, deploy, Traefik, firewall, terminal web, file manager |
| [Plataforma](plataforma.md) | Login, RBAC (admin/viewer), dashboard, notificações, verificação de atualização |
| [Segurança](seguranca.md) | O que foi endurecido e o que continua sendo trade-off consciente |

## Convenções usadas nesta documentação

- Nomes de aba na UI aparecem como **Aba** (ex.: aba **Monitoramento**).
- Comandos são pra rodar no host onde o gest-postgres está instalado, como root, salvo indicação contrária.
- "Servidor gerenciado" = uma instância PostgreSQL provisionada por esta plataforma. "Container genérico" = qualquer outro container do host, gerenciado pela seção Docker.
