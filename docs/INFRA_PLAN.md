# Plano: gestão genérica de Docker + Traefik + firewall

Pedido do usuário (2026-07-18): além de gerenciar Postgres, a plataforma deve
gerenciar Docker de forma genérica — subir containers a partir de
docker-compose.yml ou Dockerfile, controlar containers/networks/volumes
quaisquer, ver logs, subir Traefik como reverse proxy com domínio + SSL via
Let's Encrypt, e ter controle de firewall do host. Autorização explícita pra
decidir tudo sozinho e não parar até terminar.

## Por que isso é maior que qualquer feature anterior desta sessão

Cada item anterior (pooling, backup, hba.conf) estendia o domínio já
existente ("gerenciar um Postgres"). Isso aqui introduz um domínio novo
("gerenciar Docker/infra em geral") com uma implicação de segurança que os
outros não tinham: **controle de firewall do host exige privilégio que o
backend, de propósito, nunca teve** (ele só fala com o Docker via
docker-socket-proxy, nunca toca o host direto — ver seção "Docker
management" do CLAUDE.md). Resolver isso sem quebrar esse modelo é a decisão
de arquitetura central deste plano.

## Decisões de arquitetura

### 1. Containers/networks/volumes/logs genéricos
Reaproveita o `internal/docker` (Docker Engine API via docker-socket-proxy)
que já é 100% genérico por baixo — `StartContainer`/`StopContainer`/
`RemoveContainer`/`ContainerLogs` já não são específicos de Postgres, só a
camada `server` que os chama é. Nova camada `internal/infra` expõe isso sem
a amarra de "precisa ser um servidor cadastrado": qualquer container do
host, gerenciado pela plataforma ou não.

Novidade real: `NetworkList/NetworkCreate/NetworkRemove` e
`VolumeList/VolumeRemove` não existiam expostos (só criação implícita via
`EnsureNetwork`/`EnsureVolume`). Adiciona no `docker.Client`.

### 2. Deploy via docker-compose.yml
Não vale a pena reimplementar o parser de compose (orquestração de rede,
depends_on, healthcheck etc. — o próprio `docker compose` já faz isso
certo). Decisão: instala `docker-cli` + `docker-cli-compose` na imagem do
backend (que já tem `DOCKER_HOST` apontando pro docker-socket-proxy — o CLI
usa a mesma variável, sem mudança de rede) e roda `docker compose -p
<projeto> -f <arquivo> up -d` via `os/exec`, igual o `pg_dump` do backup.
Arquivos de compose + qualquer contexto de build ficam num volume novo
(`stacks_data`, montado em `/stacks`).

Todo stack implantado por aqui entra também na rede `gestpg-managed`
(override do `docker-compose.yml` enviado, força
`networks.default.external.name=gestpg-managed`) — assim o Traefik (que já
vive nessa rede) sempre consegue alcançar qualquer coisa que a plataforma
subiu, sem precisar reconectar rede toda hora.

### 3. Build a partir de Dockerfile
Mesma lógica: `docker build -f <Dockerfile> -t <tag> <contexto>` via
`os/exec`. Precisa da categoria `BUILD` no docker-socket-proxy (nova,
adicionada só agora) — **nunca `EXEC`**, que continua fora por decisão de
segurança de toda a sessão. Build de Dockerfile arbitrário é
inerentemente "rodar o que o admin mandar" — aceitável aqui porque quem
usa essa tela já é o operador confiável da própria plataforma (mesmo
modelo de confiança do Portainer/CapRover), não um usuário anônimo.

### 4. Traefik + domínio + Let's Encrypt
Container `traefik:v3` gerenciado como qualquer outro (novo
`CreateGenericContainer` no `docker.Client`, mais flexível que o
`CreateContainer` específico de Postgres — aceita portas/binds/comando
arbitrários). Publica 80 e 443. Dois providers habilitados:
- **file provider** (`/dynamic`, watch=true): é como a plataforma registra
  domínio → container:porta sem precisar tocar no container alvo (nunca
  precisa recriar nada só pra rotear um domínio novo — diferente de usar
  labels, que exigiriam recriar o container toda vez que uma rota muda).
- **docker provider** (`exposedByDefault=false`): só participa quem tiver
  label `traefik.enable=true` explícita — não expõe nada por acidente.

Let's Encrypt via HTTP-01 (`certificatesresolvers.le.acme.httpchallenge`) —
não precisa de credencial de provedor de DNS, só a porta 80 alcançável de
fora (pré-requisito que qualquer droplet público já tem). E-mail de
registro fica numa config de plataforma nova (`platform_settings`, linha
única, mesmo padrão do `gdrive_connection`).

### 5. Firewall do host — a decisão mais delicada

O backend roda em container sem `NET_ADMIN`/rede do host, de propósito
(mesma razão de nunca montar o socket do Docker direto). `ufw`/`iptables`
manipulam o namespace de rede do HOST — não tem como isso funcionar de
dentro do container do backend sem dar privilégio que quebraria o modelo
inteiro de segurança da plataforma.

Solução: **agente próprio, mínimo, rodando FORA do Docker, direto no
host**, igual em espírito ao `docker-socket-proxy` (um mediador estreito,
não a coisa toda exposta). `firewall-agent/` é um binário Go separado
(módulo próprio, fora do módulo do `backend`), instalado pelo `setup.sh`
como serviço systemd, que:
- Escuta só em socket Unix (`/run/gestpg-firewall.sock`) — nunca porta de
  rede, nem local nem pro Docker.
- Expõe só 3 operações: listar regras, liberar porta/protocolo, remover
  regra por porta/protocolo.
- **Nunca aceita remover a regra da porta 22/tcp** — trava dura no código,
  não uma confirmação de UI que dá pra pular. `ufw disable` e `ufw --force
  reset` não são operações expostas, ponto.
- `setup.sh` garante `ufw` instalado e com 22/tcp liberado ANTES de
  habilitar `ufw enable` — ordem importa, senão trava acesso SSH.

O backend recebe o socket via bind mount (`docker-compose.yml`), fala HTTP
sobre ele. Superfície de ataque: quem tem acesso ao backend ganha controle
de firewall — igual hoje quem tem acesso ao backend já ganha controle
total de qualquer Postgres gerenciado; consistente com o resto do modelo
("backend é o operador confiável", ainda sem login próprio — item #46
continua pendente, fora do escopo deste plano).

## Ordem de execução

1. Containers/networks/volumes/logs genéricos (backend + frontend) — base
   pra tudo que vem depois.
2. Deploy via compose + build via Dockerfile.
3. Traefik + domínio + Let's Encrypt.
4. Firewall agent + UI.

Cada fase: implementar → build no droplet → testar de verdade (curl +
browser) → limpar o que for de teste → commit. Mesmo ritmo usado a sessão
inteira.
