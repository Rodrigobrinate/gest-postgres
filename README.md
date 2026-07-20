# gest-postgres

Plataforma de gestão de servidores PostgreSQL em Docker. Contexto completo do
produto em [IDEIA.md](./IDEIA.md) e [REQUISITOS.md](./REQUISITOS.md). Estado
do MVP (o que já foi implementado vs o que falta) em [CLAUDE.md](./CLAUDE.md).

## Rodando local pela primeira vez

Pré-requisitos: Docker + Docker Compose, Go 1.22+ (só pra gerar o `go.sum` na
primeira vez — depois disso o build roda dentro do container).

```bash
cp .env.example .env
# editar .env: trocar METADATA_DB_PASSWORD e gerar CREDENTIAL_ENCRYPTION_KEY
# (openssl rand -hex 32)

cd backend
go mod tidy   # gera backend/go.sum — necessário 1x, não foi commitado ainda
cd ..

docker compose up --build
```

- Backend: http://localhost:28080 (`GET /api/v1/healthz`)
- Frontend: http://localhost:4173

## Estrutura

```
backend/    API em Go — provisiona/gerencia os Postgres via Docker Engine API
frontend/   Next.js — UI (lista de servidores, wizard de criação, ações)
docker-compose.yml   metadata-db + docker-socket-proxy + backend + frontend
```

O backend nunca acessa `/var/run/docker.sock` diretamente — só através do
`docker-socket-proxy`, restrito às operações de container/imagem/rede/volume
que a plataforma realmente usa.

## Acesso de fora / firewall

`PUBLIC_API_URL`/`ALLOWED_ORIGINS` no `.env` são detectados automaticamente
pelo `setup.sh` a partir do IP público — se trocar de domínio depois, atualiza
os dois (`ALLOWED_ORIGINS` é a allowlist de CORS do backend; requisição de uma
origem fora dela é bloqueada).

Porta publicada de container **passa por cima do `ufw`** por padrão (é como o
Docker lida com iptables — regra de NAT roda antes do `INPUT` do ufw). O
`setup.sh` já deixa a cadeia `DOCKER-USER` pronta pra isso funcionar; depois
que tiver domínio+Traefik na frente e quiser travar o acesso direto por
IP:porta, `ufw route deny proto tcp to any port 28080` (ou 4173) passa a
filtrar de verdade.

## Desenvolvimento sem rebuildar containers

```bash
# backend
cd backend && go run ./cmd/api

# frontend
cd frontend && cp .env.local.example .env.local && npm run dev
```
