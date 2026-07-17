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

# ---------- 3. .env ----------
if [[ -f .env ]]; then
	ok ".env já existe, não mexi"
else
	log "gerando .env a partir de .env.example"
	cp .env.example .env
	sed -i "s/^METADATA_DB_PASSWORD=.*/METADATA_DB_PASSWORD=$(openssl rand -hex 16)/" .env
	sed -i "s/^CREDENTIAL_ENCRYPTION_KEY=.*/CREDENTIAL_ENCRYPTION_KEY=$(openssl rand -hex 32)/" .env
	[[ "$REAL_USER" != "root" ]] && chown "$REAL_USER:$REAL_USER" .env
	ok ".env criado com senha do metadata-db e chave de criptografia geradas"
fi

# ---------- 4. backend/go.sum ----------
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
	[[ "$REAL_USER" != "root" ]] && chown "$REAL_USER:$REAL_USER" backend/go.mod backend/go.sum
	ok "backend/go.sum gerado"
fi

# ---------- 5. sobe o stack ----------
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
echo "  frontend: http://localhost:3000"
echo "  backend:  http://localhost:8080/api/v1/healthz"
echo "  logs:     docker compose logs -f"
[[ "$REAL_USER" != "root" ]] && echo -e "${c_yellow}!${c_reset} deslogue/logue de novo pra usar 'docker' sem sudo"
