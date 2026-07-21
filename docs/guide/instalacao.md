# Instalação

## Requisitos

- Debian ou Ubuntu (testado em droplet Debian real), acesso root/sudo.
- Domínio opcional — funciona por IP puro; domínio + HTTPS é configurado depois, via [Traefik](docker-infra.md#traefik-reverse-proxy--domínio--ssl).
- Sem Docker pré-instalado? Sem problema — `setup.sh` instala.

## Instalação em um comando

```bash
git clone https://github.com/Rodrigobrinate/gest-postgres.git && cd gest-postgres && sudo ./setup.sh
```

`setup.sh` é **idempotente** — rodar de novo não quebra nada já instalado (reconfigura o que mudou, não recria o que já está certo). Isso é o que faz um `git pull && sudo ./setup.sh` funcionar como fluxo de atualização (ver [Verificação de atualização](plataforma.md#verificação-de-atualização)).

## O que o script faz, em ordem

1. Instala Docker (se faltar) e habilita o serviço.
2. Cria `.env` a partir de `.env.example` na primeira execução; em execuções seguintes, **regenera qualquer segredo que ainda esteja no valor de exemplo** (`ADMIN_PASSWORD`, `CREDENTIAL_ENCRYPTION_KEY`) — nunca sobrescreve um segredo já customizado.
3. Aplica `chmod 600 .env` (idempotente, toda execução) — evita `.env` world-readable em host multiusuário.
4. Cria a pasta do [gerenciador de arquivos do host](docker-infra.md#gerenciador-de-arquivos) (`HOST_FILES_ROOT`, default `/srv/gestpg-files`) se não existir.
5. Builda e instala o `firewall-agent` (binário Go, roda como serviço systemd no host — nunca em container, porque `ufw` mexe no namespace de rede do host, não do container).
6. Builda e instala o `update-agent` (mesmo padrão do `firewall-agent`) — é quem executa o botão [Atualizar agora](plataforma.md#como-atualizar-agora-funciona-por-baixo) do dashboard.
7. Instala `ufw`, libera `22/tcp` **antes** de habilitar (ordem crítica — nunca corre risco de travar o próprio acesso SSH), depois habilita.
8. Aponta a cadeia `DOCKER-USER` do iptables pra `ufw-user-forward`, o que permite que regras de `ufw route allow/deny` filtrem portas publicadas por container (sem isso, `ufw` sozinho nunca enxerga tráfego que chega via porta publicada do Docker). Não bloqueia nada sozinho — só habilita o mecanismo.
9. Builda as imagens `gestpg-postgres:13` até `:17` (Postgres oficial + `pgvector` + `pg_cron` via apt, repo PGDG) — usadas por servidores novos.
10. Exporta `GIT_COMMIT=$(git rev-parse --short HEAD)` e sobe o stack: `docker compose up --build -d`.
11. Espera o backend responder em `/api/v1/healthz` (até 30s) e imprime a URL do frontend, backend e a senha de admin gerada.

## Variáveis de ambiente (`.env`)

| Variável | Default | Pra que serve |
|---|---|---|
| `METADATA_DB_USER` / `METADATA_DB_PASSWORD` / `METADATA_DB_NAME` | `gestpg` / gerado / `gestpg_metadata` | Credenciais do Postgres interno da plataforma (metadados, nunca os servidores gerenciados) |
| `CREDENTIAL_ENCRYPTION_KEY` | gerado (`openssl rand -hex 32`) | Chave usada pra cifrar todo segredo guardado no banco de metadados (senhas de servidor, token OAuth do Drive, credencial Git, token de bot Telegram) |
| `ADMIN_PASSWORD` | gerado (`openssl rand -hex 16`) | Senha do usuário `admin` — só é ecoada no terminal na primeira geração, guarde |
| `HOST_FILES_ROOT` | `/srv/gestpg-files` | Pasta do host exposta pelo gerenciador de arquivos — nunca a raiz do filesystem |
| `PUBLIC_API_URL` | `http://localhost:28080` | URL pública da API, usada pelo **navegador** (não pelo Docker interno) — trocar pro domínio/IP real em produção |
| `ALLOWED_ORIGINS` | IP público detectado automaticamente | Allowlist de CORS (esquema+host+porta exatos de onde o frontend é acessado) |
| `TRUSTED_PROXIES` | vazio | CIDRs/IPs cujo `X-Forwarded-For`/`X-Forwarded-Proto` é honrado — vazio por padrão (throttle de login/cookie `Secure` usam só `RemoteAddr`, nunca um header forjável). Só preencher se um reverse proxy de verdade (Traefik, por exemplo) estiver na frente do backend — ver [Segurança](seguranca.md) |

Alterar `PUBLIC_API_URL` ou `HOST_FILES_ROOT` depois da primeira instalação exige `docker compose up --build -d` de novo (a primeira é embutida no bundle JS do frontend em build time).

## Portas

| Serviço | Porta | Observação |
|---|---|---|
| Frontend | `4173` | Não é `3000` (default do Next.js) de propósito — evita colisão com outros serviços comuns no mesmo host |
| Backend/API | `28080` | Não é `8080` de propósito, mesmo motivo |

## Primeiro acesso

1. Abra `http://<ip-ou-domínio>:4173`.
2. Login: `admin` / senha impressa no fim do `setup.sh`.
3. Crie o primeiro servidor em **Servidores → Novo servidor**, ou use **Procurar servidores** se já existir algum Postgres em container no host (ver [auto-descoberta](servidores.md#auto-descoberta)).

> Perdeu a senha impressa? `docker compose logs backend | grep -i senha` mostra se o backend teve que gerar uma própria (só acontece se `.env` já existia de uma instalação anterior à correção que passou `ADMIN_PASSWORD` pro container — ver [Segurança](seguranca.md)).

## Recursos mínimos

Testado de ponta a ponta num droplet Debian de **1GB de RAM**. Servidores Postgres individuais usam o preset de recursos escolhido na criação (Pequeno/Médio/Grande) — o host precisa ter memória suficiente pra soma de tudo que for rodar.
