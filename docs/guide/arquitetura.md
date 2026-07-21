# Arquitetura

## Stack

| Camada | Tecnologia |
|---|---|
| Backend | Go — `docker/docker/client` (Docker Engine API), `pgx` (driver Postgres), goroutines pra polling de métricas |
| Frontend | React + Next.js + shadcn/ui + TanStack Query + Recharts/Tremor |
| Tempo real | WebSocket direto do backend Go (terminal web, algumas atualizações ao vivo) |
| Banco de metadados | Postgres separado — servidores registrados, usuários da plataforma, histórico de config, auditoria, agendamento de backup. **Não é** nenhum dos Postgres gerenciados |
| Acesso ao Docker | Nunca socket direto — sempre via `docker-socket-proxy` |

## Diagrama de containers

```
                    ┌─────────────┐
   navegador  ───▶  │  frontend   │  :4173 (Next.js)
                    └──────┬──────┘
                           │ REST + WebSocket
                           ▼
                    ┌─────────────┐        ┌──────────────────┐
                    │   backend   │───────▶│  metadata-db      │
                    │   :28080    │        │  (Postgres 16)    │
                    └──────┬──────┘        └──────────────────┘
                           │ tcp://docker-socket-proxy:2375
                           ▼
                    ┌─────────────────────┐
                    │ docker-socket-proxy │──▶ /var/run/docker.sock (ro)
                    └──────────┬──────────┘
                               │ cria/inspeciona
                               ▼
              ┌────────────────────────────────────┐
              │ containers gerenciados: Postgres,   │
              │ PgBouncer, Traefik, containers       │
              │ genéricos criados via UI             │
              └────────────────────────────────────┘

   firewall-agent (systemd, no HOST, fora do Docker) ◀── socket Unix ── backend
```

## Por que `docker-socket-proxy`

O backend **nunca** monta `/var/run/docker.sock` diretamente. Todo acesso ao Docker Engine passa por `tecnativa/docker-socket-proxy`, que expõe só as categorias de API necessárias:

```yaml
CONTAINERS: 1
IMAGES: 1
NETWORKS: 1
VOLUMES: 1
POST: 1
INFO: 1
SYSTEM: 1
BUILD: 1   # docker build / compose up --build (tela Docker: build via Dockerfile)
EXEC: 1    # terminal web + alguns comandos sem equivalente na API de archive
```

`EXEC` liga o terminal web e algumas operações que não têm equivalente na API de archive (ex.: excluir arquivo dentro de container, `find -delete` no restore de volume). É uma decisão consciente que aumenta a superfície de ataque — quem alcança a UI autenticada roda qualquer comando em qualquer container do host. Mitigado por exigir sessão autenticada em toda a API (ver [Login e RBAC](plataforma.md#login-e-rbac)) e por nunca liberar `EXEC` como caminho **padrão** — a maioria das operações de arquivo usa a API de archive do Docker (`GET`/`PUT /containers/{id}/archive`), que não é exec.

## Por que Postgres de metadados separado

O estado da própria plataforma (servidores cadastrados, usuários, histórico de config, política de backup, credenciais cifradas) vive num Postgres próprio (`metadata-db`), completamente separado dos Postgres que a plataforma gerencia. Isso evita o problema óbvio de "o banco que guarda a lista de bancos" ficar acoplado ao ciclo de vida de qualquer servidor individual.

## Redes Docker

- `gestpg-internal` — backend, metadata-db, docker-socket-proxy. Nunca alcançável de fora.
- `gestpg-managed` — todo Postgres gerenciado (e PgBouncer companheiro) entra aqui. O backend fala com cada servidor pelo **nome do container**, nunca por `host_port` — a porta publicada é só pra acesso externo (psql, DBeaver, etc.).

## Volumes nomeados (nunca bind mount do host, exceto onde documentado)

| Volume | Conteúdo |
|---|---|
| `metadata_db_data` | Dados do Postgres de metadados |
| `backups_data` | `pg_dump` quando o storage é "local" |
| `generic_backups_data` | Snapshots `.tar.gz` de volume genérico |
| `stacks_data` | `docker-compose.yml`/`Dockerfile` enviados pela tela Docker |
| `gestpg-traefik-dynamic` | Rotas do Traefik (arquivos YAML, file provider) — compartilhado entre backend (escreve) e Traefik (lê) |
| `gestpg-traefik-letsencrypt` | Certificados ACME (`acme.json`) — só o Traefik usa |
| um volume nomeado por servidor Postgres | Dados daquele Postgres — sobrevive a recriação de container |

A única pasta do **host** montada é `HOST_FILES_ROOT` (gerenciador de arquivos, ver [Docker genérico](docker-infra.md#gerenciador-de-arquivos)) e um único arquivo (`/etc/hostname`, só pra medir disco via `statfs`) — nunca a raiz do filesystem nem `/etc` inteiro.

## `firewall-agent`

Única peça que não dá pra fazer só com `docker-socket-proxy`, porque `ufw` mexe no namespace de rede do **host**, não do container. É um binário Go separado (`firewall-agent/`), roda direto no host via `systemd`, escuta só num socket Unix (`/run/gestpg-firewall.sock`, montado no backend). Só expõe listar/liberar/remover regra — nunca `ufw enable/disable/reset`, e a porta `22/tcp` (SSH) nunca pode ser tocada por essa API, travado no código do próprio agente.

## `update-agent`

Mesmo raciocínio do `firewall-agent`, pro botão [Atualizar agora](plataforma.md#como-atualizar-agora-funciona-por-baixo): `git pull` + `./setup.sh` precisam de root no host, não dá pra fazer de dentro do container do backend. Binário Go separado (`update-agent/`), roda via `systemd`, socket próprio (`/run/gestpg-update.sock`). Só dois endpoints, nenhum aceita parâmetro — a pipeline é sempre a mesma, fixa no binário.

Detalhe de arquitetura específico desse agente: a pipeline que ele dispara roda numa **unit systemd transiente separada** (`systemd-run --unit=...`), não como filho direto do processo do agente — porque o próprio `setup.sh` reinstala e reinicia o `update-agent` a cada execução (mesmo motivo do `firewall-agent`), e se a pipeline fosse filho direto, esse restart mataria a própria atualização no meio do caminho (systemd mata todo processo do cgroup do serviço ao reiniciar). Estado e log da execução ficam em arquivo em disco, não em memória, pelo mesmo motivo — sobrevivem ao agente reiniciar ou cair.

## Fluxo de configuração do Postgres

Todo parâmetro (inicial na criação, ou edição depois) passa por `ALTER SYSTEM SET` + `pg_reload_conf()` (ou restart quando o parâmetro exige). Isso corrigiu um bug de arquitetura do início do projeto: a config inicial entrava como flag `-c` no comando do container, que tem prioridade **maior** que `ALTER SYSTEM` — qualquer edição pós-criação nunca surtia efeito, nem com restart. Ver [Configuração](configuracao.md) pro detalhe de quais parâmetros pedem reload vs. restart.

## Imagem Postgres customizada

`postgres:X` oficial não vem com `pgvector` nem `pg_cron` compilados. Servidores novos sobem em cima de `gestpg-postgres:X` (`postgres-image/Dockerfile`) — a mesma imagem oficial + esses dois pacotes via apt (repo PGDG que a imagem já tem configurado). Buildada localmente pelo `setup.sh` uma vez por versão suportada (13 a 17); o backend só faz `pull`/`inspect` na criação de servidor, nunca `build` (fora do que `docker-socket-proxy` libera de propósito). Servidores criados antes dessa mudança continuam na imagem antiga, sem pgvector/pg_cron, até serem recriados — sem migração automática.
