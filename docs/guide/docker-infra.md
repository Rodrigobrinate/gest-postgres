# Gestão genérica de Docker

Seção **Docker** (`/infra`) na tela inicial, separada da gestão de servidores Postgres. Cobre qualquer container do host, não só servidores gerenciados.

## Containers, redes e volumes

- Start/stop/restart/remover **qualquer** container do host.
- Criar/remover rede e volume.
- Ver logs.

Reaproveita o mesmo `internal/docker` (Docker Engine API via `docker-socket-proxy`) que já era genérico por baixo — só a camada de `server` que era amarrada a "servidor cadastrado".

**Proteções**: nunca deixa apagar as redes/volumes fixos da própria plataforma, nem o volume de dados de um servidor gerenciado (esse tem fluxo próprio, com sua própria confirmação).

## Criar container — 4 modos

Dialog único, escolha entre:

1. **Imagem** — puxa imagem existente, com editor de variável de ambiente e porta.
2. **Dockerfile** — build a partir de um Dockerfile, com ou sem contexto extra via upload `.tar`/`.tar.gz`.
3. **Compose** — deploy via `docker-compose.yml`.
4. **Git** — clona repositório (público, ou privado via credencial SSH/PAT cadastrada) e builda a partir do Dockerfile clonado.

Compose/build chamam `docker compose`/`docker build` via `os/exec` de dentro do próprio container do backend (CLI instalado na imagem, fala pelo mesmo `DOCKER_HOST` do proxy) em vez de reimplementar orquestração de compose na unha. Categoria `BUILD` própria no `docker-socket-proxy` — `EXEC` continua sem relação com isso.

### Segurança do modo Git

- Allowlist de esquema (`http`/`https`/`ssh` + sintaxe SCP).
- Bloqueio de `::` (mata o transporte `ext::`, que executava comando arbitrário dentro do container do backend — era o item CRÍTICO/RCE do relatório de segurança).
- Bloqueio de argumento começando com `-`/`--` antes dos posicionais.
- `GIT_ALLOW_PROTOCOL` como cinto-e-suspensório.
- Token de PAT **nunca** vai no argv do `git clone` (ficaria legível via `/proc/<pid>/cmdline`) — só o username (ou `x-access-token`) vai na URL, o segredo passa por `GIT_ASKPASS` (env var).

Credenciais Git ficam cadastradas em `internal/infra/git_credentials.go`, reaproveitadas tanto no modo Git de criação de container quanto no deploy automático via webhook (abaixo).

## Limite de recursos

CPU/memória, mesmo campo que servidor Postgres já tinha — disponível nos modos Imagem/Dockerfile/Git, editável ao vivo na aba **Visão Geral** do detalhe do container (`UpdateContainerResources`).

## Página de detalhe do container

Clicar no nome na lista abre:

- Inspect completo.
- Gráfico de CPU/memória/rede — histórico **sob demanda**: só começa a coletar quando alguém abre a aba **Estatísticas**, para sozinho depois de 10min sem leitura (diferente do histórico da plataforma, sempre ligado — "todo container do host" não tem limite razoável pra manter sempre coletando).
- Variáveis de ambiente.
- Logs.
- Redes — conectar/desconectar ao vivo.
- Volumes.
- **Terminal** (xterm.js + WebSocket) — exige categoria `EXEC` no `docker-socket-proxy` (decisão consciente, ver [Arquitetura](arquitetura.md#por-que-docker-socket-proxy)).
- **Cron** — roda um comando shell (`sh -c`) num horário/intervalo. Mesmo "cron básico" do backup de Postgres: intervalo em minutos OU diário/semanal + horário UTC, sem parser de cron de verdade. Depende de `EXEC`.

## Gerenciador de arquivos

Dois sabores:

### Dentro de container ou volume

Usa a API de archive do Docker (mesmo mecanismo do `pg_hba.conf`, sem `exec`) pra listar/ler/escrever/upload/baixar. **Exclusão** é a única operação sem equivalente na API de archive — cai pro `exec` síncrono (`rm -rf`), dependência documentada de propósito.

Volume solto (sem container associado) é gerenciado subindo um container `alpine` descartável com o volume montado, roda a operação, remove — mesmo mecanismo por baixo, sem segundo caminho de código.

### Do host

Pasta fixa/configurável (`HOST_FILES_ROOT`, default `/srv/gestpg-files`, **nunca a raiz `/`**) montada read-write no backend.

- Listar/ler/baixar não exige nada extra.
- **Escrever/enviar/excluir exige reconfirmar a senha** antes (`POST /api/v1/auth/step-up`, eleva a sessão por 5 minutos).

## Backup de volume genérico

Botão na aba **Volumes** — snapshot manual `.tar.gz` de qualquer volume nomeado, reaproveitando o mesmo container `alpine` descartável (lê via API de archive, comprime em streaming direto pro disco). Guardado em `generic_backups_data` (volume próprio, nunca bind mount do host). **Sem agendamento** (diferente do backup de Postgres) — só manual.

**Restaurar** — mesmo padrão de 2 modos do restore de backup Postgres: criar volume novo ou sobrescrever existente. Sobrescrever limpa o volume via `exec` (`find -mindepth 1 -delete`) antes de extrair o tar — Docker não tem operação de archive API pra "limpar diretório".

## Deploy automático via webhook

Persiste a config de clone+build do modo Git (`git_deployments`) e reimplanta o container sozinho a cada push.

> **Decisão de segurança relevante**: o endpoint de webhook (`POST .../git-deployments/{id}/webhook`) é a única rota de toda a API que **não passa por sessão de usuário** — GitHub/GitLab não têm como mandar cookie de login. Autenticação ali é só a assinatura do provedor: HMAC-SHA256 `X-Hub-Signature-256` (GitHub) ou comparação direta de `X-Gitlab-Token` (GitLab), contra um segredo aleatório de 32 bytes gerado por deployment, cifrado no banco, devolvido em texto puro só uma vez na criação. Padrão de mercado pra webhook — nenhum provedor consegue autenticar via sessão — mas ainda assim é a única rota pública da API. A rota casa o padrão **exato**, não sufixo, pra nunca expor por engano uma rota futura terminando em `/webhook`.

## Traefik (reverse proxy + domínio + SSL)

Container gerenciado, rotas via **file provider** (arquivo YAML por domínio) — nunca precisa recriar o container alvo pra rotear. ACME HTTP-01 (só precisa da porta 80 alcançável, sem credencial de DNS).

Recursos por rota:

- Domínio → container gerenciado por esta plataforma.
- Domínio → **destino externo** (IP/host fora do Docker gerenciado por esta plataforma) — `target_url`, mutuamente exclusivo com proxy-pra-container e redirect.
- **Path prefix** + strip.
- **Modo redirecionamento** (301/302 configurável).
- **Redirecionamento automático http→https** quando TLS está ligado — também conserta um bug antigo: sem essa opção, com TLS ligado a porta 80 não tinha router pra aquele domínio e dava 404 em vez de redirecionar.

E-mail do Let's Encrypt é **global** (1 por instalação, definido ao habilitar o Traefik) — arquitetura do ACME/Traefik não separa e-mail por domínio.

Fora de escopo, de propósito: **TCP/UDP passthrough** (exigiria recriar o container do Traefik, interrompendo todo roteamento HTTP, só pra abrir um entrypoint novo).

### Reuso de Traefik já existente no host

Se o host já roda outro Traefik (ex.: EasyPanel) e o `gestpg-traefik` próprio **não** está habilitado, a tela de rotas opera em **modo "via labels"**: aplica os labels que o provider Docker do Traefik externo entende direto no container **alvo** (nunca no Traefik em si), reaproveitando o mesmo mecanismo de "recriar só o alvo" já usado pra env var/volume — que primeiro remove qualquer label `traefik.*` antigo e depois aplica os novos (um container sustenta uma rota via label por vez nessa versão). Best-effort: conecta o alvo em toda rede que o Traefik externo já está. `certResolver` do TLS é campo opcional que o usuário preenche — a plataforma não descobre sozinha o nome do resolver ACME já configurado ali.

O provider Docker do próprio Traefik gerenciado por esta plataforma foi cortado de propósito — exigiria dar a ele leitura no Docker que ele não tem por design, e o file provider já cobre tudo.

## Firewall do host (ufw)

Ver [Arquitetura → firewall-agent](arquitetura.md#firewall-agent) pra como funciona por baixo.

Na UI: listar/liberar/remover regra, com **origem** (IP/CIDR) opcional (`ufw allow from X to any port Y`), não só porta/protocolo/ação.

Peculiaridades:

- `ufw status` não mostra nada enquanto o firewall está **inativo**, mesmo com regra já gravada (`ufw show added` confirma que existe) — o agente cai pro `show added` quando detecta `ufw` inativo, senão a listagem mentia "vazio" antes do primeiro `ufw enable`.
- `ufw route allow/deny` filtra porta **publicada por container** — precisa da cadeia `DOCKER-USER` apontada pra `ufw-user-forward` (feito pelo `setup.sh`, ver [Instalação](instalacao.md)). Sem isso, porta publicada (28080/4173) sempre passa direto por cima do `ufw`.

## Limpeza de sistema

`docker system prune -f` — de propósito **sem** `-a` nem `--volumes`: nunca toca volume nomeado (evita apagar dado de servidor parado por engano) nem imagem sem tag ainda referenciada por histórico de build.

## Notificações

`notification_channels` — Telegram (bot) e webhook genérico, cadastrados uma vez e referenciados por qualquer regra de alerta de qualquer servidor, em vez de colar a mesma URL/token em cada regra. Telegram vira mensagem de texto formatada via `sendMessage` do bot; regra sem canal ainda aceita URL direta (compatibilidade com o formato anterior).

`webhook_url` (canal e regra direta) tem guarda de SSRF: resolve o host e recusa loopback/link-local/privado (bloqueia `169.254.169.254` de metadata de nuvem, `localhost`, etc.); o cliente HTTP interno não segue redirect.
