#!/usr/bin/env bash
# Setup do gest-postgres em Debian (também funciona em derivados: Ubuntu etc).
# Instala Docker Engine + Compose plugin, gera .env e backend/go.sum, sobe o stack.
#
# Uso:
#   sudo ./setup.sh
#
# Idempotente: pode rodar de novo sem quebrar nada já instalado/gerado.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# ---------- logging ----------
c_blue="\033[1;34m"; c_green="\033[1;32m"; c_yellow="\033[1;33m"; c_red="\033[1;31m"; c_reset="\033[0m"
log()  { echo -e "${c_blue}==>${c_reset} $*"; }
ok()   { echo -e "${c_green}✓${c_reset} $*"; }
warn() { echo -e "${c_yellow}!${c_reset} $*"; }
die()  { echo -e "${c_red}✗ $*${c_reset}" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "roda como root: sudo ./setup.sh"

if [[ ! -f /etc/os-release ]] || ! grep -qiE '^ID(_LIKE)?=.*(debian)' /etc/os-release; then
	warn "não parece ser Debian/derivado — seguindo mesmo assim, pode falhar no apt-get"
fi

REAL_USER="${SUDO_USER:-$(logname 2>/dev/null || echo root)}"

# ---------- 1. dependências de sistema ----------
log "atualizando apt e instalando dependências base"
apt-get update -y
apt-get install -y ca-certificates curl openssl git

# ---------- 2. Docker Engine + Compose plugin ----------
if command -v docker >/dev/null 2>&1; then
	ok "docker já instalado ($(docker --version))"
else
	log "instalando Docker Engine (repositório oficial docker.com)"
	install -m 0755 -d /etc/apt/keyrings
	curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
	chmod a+r /etc/apt/keyrings/docker.asc

	. /etc/os-release
	echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian ${VERSION_CODENAME} stable" \
		> /etc/apt/sources.list.d/docker.list

	apt-get update -y
	apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
	ok "docker instalado ($(docker --version))"
fi

systemctl enable --now docker
ok "serviço docker ativo"

if [[ "$REAL_USER" != "root" ]] && ! id -nG "$REAL_USER" | grep -qw docker; then
	usermod -aG docker "$REAL_USER"
	warn "usuário '$REAL_USER' adicionado ao grupo docker — precisa deslogar/logar de novo pra valer sem sudo"
fi

DOCKER="docker"
command -v docker >/dev/null || die "docker não ficou disponível no PATH"

# ---------- 3. swap ----------
# Droplets pequenos (512MB-1GB) costumam OOMar no build do frontend (next build)
# ou do backend (go build) sem isso. Só mexe se a máquina realmente tem pouca RAM.
SWAP_SIZE_MB=2048
if swapon --show --noheadings | grep -q .; then
	ok "swap já ativo"
elif [[ -f /swapfile ]]; then
	log "ativando /swapfile existente"
	swapon /swapfile
	ok "swap ativo"
else
	MEM_MB=$(awk '/MemTotal/ {print int($2/1024)}' /proc/meminfo)
	if [[ $MEM_MB -lt 1536 ]]; then
		log "RAM baixa (${MEM_MB}MB) — criando swapfile de ${SWAP_SIZE_MB}MB pra build não OOMar"
		fallocate -l "${SWAP_SIZE_MB}M" /swapfile 2>/dev/null || dd if=/dev/zero of=/swapfile bs=1M count=$SWAP_SIZE_MB status=none
		chmod 600 /swapfile
		mkswap /swapfile >/dev/null
		swapon /swapfile
		grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
		ok "swap de ${SWAP_SIZE_MB}MB ativo"
	else
		ok "RAM (${MEM_MB}MB) suficiente, sem swap"
	fi
fi

# ---------- 4. .env ----------
if [[ -f .env ]]; then
	ok ".env já existe, não mexi"
else
	log "gerando .env a partir de .env.example"
	cp .env.example .env
	sed -i "s/^METADATA_DB_PASSWORD=.*/METADATA_DB_PASSWORD=$(openssl rand -hex 16)/" .env
	sed -i "s/^CREDENTIAL_ENCRYPTION_KEY=.*/CREDENTIAL_ENCRYPTION_KEY=$(openssl rand -hex 32)/" .env
	sed -i "s/^ADMIN_PASSWORD=.*/ADMIN_PASSWORD=$(openssl rand -hex 16)/" .env

	# PUBLIC_API_URL vira NEXT_PUBLIC_API_URL embutido no bundle JS do frontend em
	# build time — precisa ser o IP/domínio que o NAVEGADOR do usuário alcança, não
	# "localhost" (que resolveria pro localhost do PC de quem tá acessando, não do
	# servidor). Detecta o IP público automaticamente; sem internet, fica localhost
	# mesmo (funciona só pra acessar de dentro da própria máquina).
	PUBLIC_IP="$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
	if [[ -n "$PUBLIC_IP" ]]; then
		sed -i "s#^PUBLIC_API_URL=.*#PUBLIC_API_URL=http://${PUBLIC_IP}:8080#" .env
		ok "PUBLIC_API_URL detectado automaticamente: http://${PUBLIC_IP}:8080"
	else
		warn "não consegui detectar IP público — PUBLIC_API_URL ficou localhost:8080 (edita o .env se for acessar de fora)"
	fi

	[[ "$REAL_USER" != "root" ]] && chown "$REAL_USER:$REAL_USER" .env || true
	ok ".env criado com senha do metadata-db e chave de criptografia geradas"
fi

# ---------- 4.5. pasta do gerenciador de arquivos do host ----------
# HOST_FILES_ROOT é a raiz (fora do container) que a aba "Arquivos do host"
# expõe — precisa existir ANTES do `docker compose up`, senão o Docker cria
# ela sozinho como root:root na hora do bind mount.
HOST_FILES_ROOT_VALUE="$(grep -m1 '^HOST_FILES_ROOT=' .env | cut -d= -f2-)"
HOST_FILES_ROOT_VALUE="${HOST_FILES_ROOT_VALUE:-/srv/gestpg-files}"
if [[ -d "$HOST_FILES_ROOT_VALUE" ]]; then
	ok "pasta do gerenciador de arquivos já existe ($HOST_FILES_ROOT_VALUE)"
else
	log "criando pasta do gerenciador de arquivos ($HOST_FILES_ROOT_VALUE)"
	mkdir -p "$HOST_FILES_ROOT_VALUE"
	[[ "$REAL_USER" != "root" ]] && chown "$REAL_USER:$REAL_USER" "$HOST_FILES_ROOT_VALUE" || true
	ok "pasta criada"
fi

# ---------- 5. backend/go.sum ----------
# Gera dentro de um container golang efêmero — não precisa instalar Go no host.
if [[ -f backend/go.sum ]]; then
	ok "backend/go.sum já existe, não mexi"
else
	log "gerando backend/go.sum (via container golang:1.22, precisa internet)"
	GO_MOD_VERSION="$(grep -m1 '^go ' backend/go.mod | awk '{print $2}')"
	"$DOCKER" run --rm \
		-v "$SCRIPT_DIR/backend:/src" \
		-w /src \
		-e GOTOOLCHAIN=auto \
		"golang:${GO_MOD_VERSION}" \
		go mod tidy
	[[ "$REAL_USER" != "root" ]] && chown "$REAL_USER:$REAL_USER" backend/go.mod backend/go.sum || true
	ok "backend/go.sum gerado"
fi

# ---------- 6. imagens Postgres gerenciadas (pgvector + pg_cron) ----------
# postgres:X oficial não vem com extensões extra compiladas. Builda local, uma
# vez por versão suportada — o backend só faz pull/inspect (nunca build) pela
# permissão restrita do docker-socket-proxy, então essas imagens precisam já
# existir localmente antes do primeiro "criar servidor".
log "buildando imagens gestpg-postgres:{13..17} (pgvector + pg_cron)"
for v in 13 14 15 16 17; do
	if "$DOCKER" image inspect "gestpg-postgres:${v}" >/dev/null 2>&1; then
		ok "gestpg-postgres:${v} já existe, não rebuildei"
		continue
	fi
	"$DOCKER" build --build-arg "PG_MAJOR=${v}" -t "gestpg-postgres:${v}" ./postgres-image
	ok "gestpg-postgres:${v} buildada"
done

# ---------- 6.5. firewall-agent (ufw) ----------
# Roda no HOST via systemd, NUNCA em container — ufw mexe no namespace de
# rede do host, não tem como isso funcionar de dentro de um container sem
# dar privilégio que quebraria o modelo de segurança do resto da plataforma
# (o backend nunca toca o host direto, só via mediadores estreitos — mesmo
# raciocínio do docker-socket-proxy). Escuta só num socket Unix local
# (/run/gestpg-firewall.sock), nunca porta de rede. Superfície mínima de
# propósito: só lista/libera/remove regra — nunca expõe ufw
# enable/disable/reset, e a porta 22/tcp nunca pode ser alterada por essa
# API (travado no código do agente, não só aqui).
log "buildando e instalando firewall-agent (systemd, roda fora do Docker)"
"$DOCKER" run --rm \
	-v "$SCRIPT_DIR/firewall-agent:/src" \
	-w /src \
	-e GOTOOLCHAIN=auto \
	-e CGO_ENABLED=0 \
	golang:1.25-alpine \
	go build -o /src/gestpg-firewall-agent .
install -m 0755 firewall-agent/gestpg-firewall-agent /usr/local/bin/gestpg-firewall-agent
rm -f firewall-agent/gestpg-firewall-agent

cat > /etc/systemd/system/gestpg-firewall-agent.service <<'UNIT'
[Unit]
Description=gest-postgres firewall agent (ufw via socket Unix local)
After=network.target

[Service]
ExecStart=/usr/local/bin/gestpg-firewall-agent
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable gestpg-firewall-agent
# restart (não só enable --now) — o binário acima é sempre reinstalado do
# zero, mas se o serviço já tava rodando "enable --now" não recarrega o
# processo com o binário novo, ele só garante habilitado+rodando (fica
# preso na versão antiga em memória até um restart de verdade).
systemctl restart gestpg-firewall-agent
ok "firewall-agent ativo (/run/gestpg-firewall.sock)"

# ufw: instala + libera 22/tcp ANTES de habilitar — ORDEM CRÍTICA, nunca
# inverter (habilitar antes de liberar SSH tranca acesso remoto ao servidor
# pra sempre, só recuperável pelo console web da provedora).
if ! command -v ufw >/dev/null 2>&1; then
	log "instalando ufw"
	apt-get install -y ufw
fi
ufw allow 22/tcp >/dev/null
if ufw status | grep -q "Status: active"; then
	ok "ufw já ativo"
else
	log "habilitando ufw (22/tcp já liberado antes, SSH nunca fica bloqueado)"
	ufw --force enable
	ok "ufw ativo"
fi

# ---------- 7. sobe o stack ----------
log "subindo o stack (docker compose up --build -d)"
"$DOCKER" compose up --build -d

log "esperando backend responder em /api/v1/healthz"
for i in $(seq 1 30); do
	if curl -fsS http://localhost:8080/api/v1/healthz >/dev/null 2>&1; then
		ok "backend no ar"
		break
	fi
	[[ $i -eq 30 ]] && warn "backend não respondeu em 30s — checa 'docker compose logs backend'"
	sleep 1
done

echo
ok "setup concluído"
echo "  frontend: http://localhost:4173"
echo "  backend:  http://localhost:8080/api/v1/healthz"
echo "  logs:     docker compose logs -f"
ADMIN_PASSWORD_SET="$(grep -m1 '^ADMIN_PASSWORD=' .env | cut -d= -f2-)"
if [[ -n "$ADMIN_PASSWORD_SET" && "$ADMIN_PASSWORD_SET" != "troque-esta-senha" ]]; then
	echo "  login:    admin / ${ADMIN_PASSWORD_SET}  (guarda essa senha, só aparece aqui)"
else
	warn "ADMIN_PASSWORD não gerada (instalação existente de antes dessa versão) — o backend gera uma sozinha no primeiro boot, checa 'docker compose logs backend | grep -i senha'"
fi
[[ "$REAL_USER" != "root" ]] && echo -e "${c_yellow}!${c_reset} deslogue/logue de novo pra usar 'docker' sem sudo" || true
