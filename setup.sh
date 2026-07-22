#!/usr/bin/env bash
# Setup do gest-postgres em Debian (também funciona em derivados: Ubuntu etc).
# Instala Docker Engine + Compose plugin, gera .env e backend/go.sum, sobe o stack.
#
# Uso:
#   sudo ./setup.sh
#   sudo ./setup.sh --cloud-token <TUNNEL_TOKEN> --integration-key <CHAVE>   # conecta ao sistema mestre na Cloudflare: derruba frontend local, tranca porta, sobe cloudflared
#   sudo ./setup.sh --cloud-disconnect                                       # reverte pro modo local (frontend + porta publicada de volta)
#
# Idempotente: pode rodar de novo sem quebrar nada já instalado/gerado. Uma
# vez em modo cloud (via --cloud-token), re-rodar SEM flag nenhuma preserva
# o modo cloud (fica gravado em CLOUD_MODE no .env) — só --cloud-disconnect
# reverte.

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

# ---------- 0. flags de modo cloud ----------
CLOUD_TOKEN=""
CLOUD_INTEGRATION_KEY=""
CLOUD_DISCONNECT=0
while [[ $# -gt 0 ]]; do
	case "$1" in
		--cloud-token)
			CLOUD_TOKEN="${2:-}"; shift 2 ;;
		--integration-key)
			CLOUD_INTEGRATION_KEY="${2:-}"; shift 2 ;;
		--cloud-disconnect)
			CLOUD_DISCONNECT=1; shift ;;
		*)
			die "argumento desconhecido: $1 (uso: --cloud-token <token> --integration-key <chave> | --cloud-disconnect)" ;;
	esac
done
if [[ ( -n "$CLOUD_TOKEN" && -z "$CLOUD_INTEGRATION_KEY" ) || ( -z "$CLOUD_TOKEN" && -n "$CLOUD_INTEGRATION_KEY" ) ]]; then
	die "--cloud-token e --integration-key precisam vir os dois juntos"
fi
if [[ -n "$CLOUD_TOKEN" && "$CLOUD_DISCONNECT" == "1" ]]; then
	die "--cloud-token e --cloud-disconnect são mutuamente exclusivos"
fi

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
ENV_JUST_CREATED=0
if [[ -f .env ]]; then
	ok ".env já existe, não recriei"
else
	log "gerando .env a partir de .env.example"
	cp .env.example .env
	ENV_JUST_CREATED=1
fi

# Troca qualquer segredo que ainda esteja no valor placeholder de
# .env.example — roda SEMPRE, não só na criação, pra fechar o caso de
# alguém ter feito `cp .env.example .env` na mão (ver README, caminho de
# dev local) e nunca trocado os placeholders, ou de um .env de instalação
# antiga de antes dessa checagem existir. Nunca mexe num segredo que já foi
# customizado — só substitui o valor de exemplo conhecido, e sempre com
# `openssl rand`, nunca uma string previsível.
if grep -q '^METADATA_DB_PASSWORD=troque-esta-senha$' .env; then
	sed -i "s/^METADATA_DB_PASSWORD=.*/METADATA_DB_PASSWORD=$(openssl rand -hex 16)/" .env
	ok "METADATA_DB_PASSWORD gerada (estava no valor de exemplo)"
fi
if grep -qE '^CREDENTIAL_ENCRYPTION_KEY=0{64}$' .env; then
	sed -i "s/^CREDENTIAL_ENCRYPTION_KEY=.*/CREDENTIAL_ENCRYPTION_KEY=$(openssl rand -hex 32)/" .env
	ok "CREDENTIAL_ENCRYPTION_KEY gerada (estava no valor de exemplo)"
fi
if grep -q '^ADMIN_PASSWORD=troque-esta-senha$' .env; then
	sed -i "s/^ADMIN_PASSWORD=.*/ADMIN_PASSWORD=$(openssl rand -hex 16)/" .env
	ok "ADMIN_PASSWORD gerada (estava no valor de exemplo)"
fi

if [[ "$ENV_JUST_CREATED" == "1" ]]; then
	# PUBLIC_API_URL vira NEXT_PUBLIC_API_URL embutido no bundle JS do frontend em
	# build time — precisa ser o IP/domínio que o NAVEGADOR do usuário alcança, não
	# "localhost" (que resolveria pro localhost do PC de quem tá acessando, não do
	# servidor). Detecta o IP público automaticamente; sem internet, fica localhost
	# mesmo (funciona só pra acessar de dentro da própria máquina). Só na criação —
	# rodar de novo aqui sobrescreveria um domínio/ALLOWED_ORIGINS já customizado.
	PUBLIC_IP="$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
	if [[ -n "$PUBLIC_IP" ]]; then
		sed -i "s#^PUBLIC_API_URL=.*#PUBLIC_API_URL=http://${PUBLIC_IP}:28080#" .env
		ok "PUBLIC_API_URL detectado automaticamente: http://${PUBLIC_IP}:28080"
		# ALLOWED_ORIGINS é a allowlist de CORS do backend — sem isso o
		# navegador acessando pelo IP público não conseguiria falar com a API
		# (allowlist ficaria só com localhost). Mantém localhost também, pra
		# continuar funcionando acessando de dentro da própria máquina.
		sed -i "s#^ALLOWED_ORIGINS=.*#ALLOWED_ORIGINS=http://${PUBLIC_IP}:4173,http://localhost:4173#" .env
		ok "ALLOWED_ORIGINS detectado automaticamente: http://${PUBLIC_IP}:4173"
	else
		warn "não consegui detectar IP público — PUBLIC_API_URL/ALLOWED_ORIGINS ficaram localhost (edita o .env se for acessar de fora)"
	fi
	ok ".env criado com senha do metadata-db e chave de criptografia geradas"
fi
[[ "$REAL_USER" != "root" ]] && chown "$REAL_USER:$REAL_USER" .env || true
# Idempotente fora do bloco de criação — corrige a permissão mesmo em
# reinstalação de um .env que já existia de antes dessa checagem (senão fica
# world-readable num host multiusuário: CREDENTIAL_ENCRYPTION_KEY decifra
# todo segredo guardado, ADMIN_PASSWORD dá login de admin).
chmod 600 .env

# ---------- 4.1. modo cloud (--cloud-token / --cloud-disconnect) ----------
# Só grava no .env aqui — quem de fato sobe o cloudflared e semeia a chave de
# integração é o próprio backend no boot (lê CLOUDFLARE_TUNNEL_TOKEN/
# INTEGRATION_KEY_SEED uma vez, ver cmd/api/main.go), reusando o mesmo canal
# de confiança que ADMIN_PASSWORD já usa — sem precisar o setup.sh fazer
# login/curl contra uma API que pode nem estar de pé ainda.
if [[ -n "$CLOUD_TOKEN" ]]; then
	log "modo cloud: gravando token do túnel e chave de integração no .env"
	grep -q '^CLOUD_MODE=' .env && sed -i "s/^CLOUD_MODE=.*/CLOUD_MODE=1/" .env || echo "CLOUD_MODE=1" >> .env
	grep -q '^CLOUDFLARE_TUNNEL_TOKEN=' .env && sed -i "s#^CLOUDFLARE_TUNNEL_TOKEN=.*#CLOUDFLARE_TUNNEL_TOKEN=${CLOUD_TOKEN}#" .env || echo "CLOUDFLARE_TUNNEL_TOKEN=${CLOUD_TOKEN}" >> .env
	grep -q '^INTEGRATION_KEY_SEED=' .env && sed -i "s#^INTEGRATION_KEY_SEED=.*#INTEGRATION_KEY_SEED=${CLOUD_INTEGRATION_KEY}#" .env || echo "INTEGRATION_KEY_SEED=${CLOUD_INTEGRATION_KEY}" >> .env
	grep -q '^BACKEND_PORT_PUBLISH=' .env && sed -i "s#^BACKEND_PORT_PUBLISH=.*#BACKEND_PORT_PUBLISH=127.0.0.1:28080:28080#" .env || echo "BACKEND_PORT_PUBLISH=127.0.0.1:28080:28080" >> .env
	ok "modo cloud gravado no .env (porta do backend vai ficar só em loopback)"
elif [[ "$CLOUD_DISCONNECT" == "1" ]]; then
	log "desconectando do modo cloud: restaurando frontend local e porta publicada"
	grep -q '^CLOUD_MODE=' .env && sed -i "s/^CLOUD_MODE=.*/CLOUD_MODE=0/" .env || echo "CLOUD_MODE=0" >> .env
	grep -q '^CLOUDFLARE_TUNNEL_TOKEN=' .env && sed -i "s/^CLOUDFLARE_TUNNEL_TOKEN=.*/CLOUDFLARE_TUNNEL_TOKEN=/" .env || true
	grep -q '^INTEGRATION_KEY_SEED=' .env && sed -i "s/^INTEGRATION_KEY_SEED=.*/INTEGRATION_KEY_SEED=/" .env || true
	grep -q '^BACKEND_PORT_PUBLISH=' .env && sed -i "s#^BACKEND_PORT_PUBLISH=.*#BACKEND_PORT_PUBLISH=28080:28080#" .env || echo "BACKEND_PORT_PUBLISH=28080:28080" >> .env
	ok "modo cloud desligado no .env"
fi
chmod 600 .env

# CLOUD_MODE_VALUE reflete o .env JÁ ATUALIZADO acima — usado mais adiante
# pra decidir profile do compose, regra de ufw e derrubar o frontend.
# Ausente (instalação de antes dessa versão) = "0", mesmo default de sempre.
CLOUD_MODE_VALUE="$(grep -m1 '^CLOUD_MODE=' .env 2>/dev/null | cut -d= -f2-)"
CLOUD_MODE_VALUE="${CLOUD_MODE_VALUE:-0}"

# ---------- 4.2. sub-rede fixa (migração de .env de instalação antiga) ----------
# .env de instalação de antes dessa versão não tem essas 2 variáveis — sem
# isso, docker-compose.yml caía no default (${VAR:-10.77...}) mesmo assim,
# mas grava explícito no .env pra ficar visível/editável, e pra o preflight
# check abaixo ler o mesmo valor que o compose vai usar.
grep -q '^GESTPG_INTERNAL_SUBNET=' .env || echo "GESTPG_INTERNAL_SUBNET=10.77.0.0/24" >> .env
grep -q '^GESTPG_MANAGED_SUBNET=' .env || echo "GESTPG_MANAGED_SUBNET=10.77.16.0/20" >> .env

GESTPG_INTERNAL_SUBNET_VALUE="$(grep -m1 '^GESTPG_INTERNAL_SUBNET=' .env | cut -d= -f2-)"
GESTPG_MANAGED_SUBNET_VALUE="$(grep -m1 '^GESTPG_MANAGED_SUBNET=' .env | cut -d= -f2-)"

# check_subnet_free: manda um IP de teste dentro da faixa pro `ip route
# get` do kernel e compara a interface que ele escolheu com a interface da
# rota DEFAULT. Se bater com a default, ninguém reivindica aquele endereço
# especificamente ainda (livre). Se vier diferente, alguma rota mais
# específica já existe pra aquele espaço — colisão.
#
# Achado em produção, motivo dessa checagem existir: sem subnet fixa nem
# checagem, Docker alocou do pool default (172.17-172.31.0.0/16) bem em cima
# da faixa 172.20.1.0/24 que o Zabbix de um usuário real já usava pra
# alcançar o que monitorava — derrubou a coleta inteira, silenciosamente,
# só descoberto porque o Zabbix parou de reportar. Subnet fixa (10.77.0.0/16
# por padrão, longe do pool do Docker e das faixas mais comuns de VPN/LAN)
# reduz a chance, mas só ISSO não é garantia nenhuma nesse host específico —
# essa checagem que é a garantia: aborta em vez de seguir silencioso.
check_subnet_free() {
	local subnet="$1"
	local base="${subnet%%/*}"
	local probe_ip
	probe_ip="$(echo "$base" | awk -F. '{print $1"."$2"."$3"."($4+1)}')"
	local probe_dev default_dev
	probe_dev="$(ip route get "$probe_ip" 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="dev") {print $(i+1); exit}}')"
	default_dev="$(ip route show default 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="dev") {print $(i+1); exit}}')"
	[[ -z "$probe_dev" || "$probe_dev" == "$default_dev" ]]
}

if command -v ip >/dev/null 2>&1; then
	log "conferindo se as sub-redes do docker-compose já não estão em uso no host"
	for subnet in "$GESTPG_INTERNAL_SUBNET_VALUE" "$GESTPG_MANAGED_SUBNET_VALUE"; do
		if ! check_subnet_free "$subnet"; then
			die "sub-rede $subnet já parece estar em uso nesse host (rota específica encontrada, diferente da rota default) — troca GESTPG_INTERNAL_SUBNET/GESTPG_MANAGED_SUBNET no .env pra uma faixa livre antes de continuar. Isso já derrubou serviço de monitoramento de um usuário — NUNCA sobe com uma faixa que colide"
		fi
	done
	ok "sub-redes $GESTPG_INTERNAL_SUBNET_VALUE e $GESTPG_MANAGED_SUBNET_VALUE livres nesse host"
else
	warn "comando 'ip' não encontrado — não consegui checar colisão de sub-rede, seguindo mesmo assim (raro nesse tipo de host)"
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

# ---------- 6.6. update-agent (botão "Atualizar agora") ----------
# Mesmo raciocínio do firewall-agent: roda no HOST via systemd, nunca em
# container, porque a pipeline que ele dispara (git pull + ./setup.sh)
# precisa rodar como root no host, não dentro do sandbox do backend. A
# pipeline é FIXA (não aceita comando arbitrário vindo da API) — ver
# update-agent/main.go pro racional completo de por que a execução em si
# roda numa unit systemd transiente separada (sobrevive a este agente
# reiniciar no meio do próprio update que ele mesmo disparou).
if ! command -v systemd-run >/dev/null 2>&1; then
	warn "systemd-run não encontrado — botão 'Atualizar agora' não vai funcionar (checagem de atualização continua funcionando normalmente)"
fi
log "buildando e instalando update-agent (systemd, roda fora do Docker)"
"$DOCKER" run --rm \
	-v "$SCRIPT_DIR/update-agent:/src" \
	-w /src \
	-e GOTOOLCHAIN=auto \
	-e CGO_ENABLED=0 \
	golang:1.25-alpine \
	go build -o /src/gestpg-update-agent .
install -m 0755 update-agent/gestpg-update-agent /usr/local/bin/gestpg-update-agent
rm -f update-agent/gestpg-update-agent

cat > /etc/systemd/system/gestpg-update-agent.service <<UNIT
[Unit]
Description=gest-postgres update agent (git pull + setup.sh via socket Unix local)
After=network.target

[Service]
Environment=GESTPG_REPO_DIR=$SCRIPT_DIR
ExecStart=/usr/local/bin/gestpg-update-agent
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable gestpg-update-agent
# restart, mesmo motivo do firewall-agent acima: o binário é sempre
# reinstalado do zero, "enable --now" sozinho não pega código novo se o
# serviço já tava rodando.
systemctl restart gestpg-update-agent
ok "update-agent ativo (/run/gestpg-update.sock)"

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

# Docker insere as próprias regras de publicação de porta nas cadeias
# nat/DOCKER do iptables ANTES do INPUT do ufw — então "ufw" sozinho NUNCA
# filtra porta publicada por container (28080/4173 ficam alcançáveis da
# internet mesmo com ufw ativo, com ou sem regra pra elas). Isso NÃO
# desbloqueia nada sozinho — só cria o caminho pra ufw conseguir enxergar
# esse tráfego, via a cadeia DOCKER-USER (Docker cria automaticamente desde
# 17.06, mas nunca com nada dentro, então nunca filtra nada por padrão).
# Idempotente: sempre reseta a cadeia antes de recriar, seguro rodar de novo.
# Cobre IPv4 e IPv6 (ip6tables) — só IPv4 deixava `ufw route deny` mudo pra
# tráfego IPv6. Depois disso, `ufw route allow/deny proto tcp to any port
# 28080` passa a funcionar de verdade (documentado no README) — não
# habilitado por padrão aqui pra não quebrar o acesso direto via IP:porta
# que o resto do setup já promete "funciona de cara" sem precisar
# configurar Traefik/domínio antes.
#
# Instalado como script próprio (não só inline aqui) porque precisa rodar
# de novo TODA vez que o Docker reinicia — `dockerd` recria a cadeia
# DOCKER-USER do zero no boot, sem nada dentro, então o salto pro
# ufw-user-forward some silenciosamente até o próximo `sudo ./setup.sh`. Uma
# unit systemd oneshot com `After=docker.service` reaplica isso sozinho a
# cada boot.
cat > /usr/local/bin/gestpg-docker-user-forward.sh <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
for bin in iptables ip6tables; do
	command -v "$bin" >/dev/null 2>&1 || continue
	"$bin" -N DOCKER-USER 2>/dev/null || true
	"$bin" -F DOCKER-USER
	"$bin" -A DOCKER-USER -j ufw-user-forward 2>/dev/null || true
	"$bin" -A DOCKER-USER -j RETURN
done
SCRIPT
chmod 0755 /usr/local/bin/gestpg-docker-user-forward.sh

if command -v iptables >/dev/null 2>&1; then
	log "habilitando cadeia DOCKER-USER (deixa ufw conseguir filtrar porta publicada por container)"
	/usr/local/bin/gestpg-docker-user-forward.sh
	ok "DOCKER-USER -> ufw-user-forward configurado (ufw route allow/deny agora filtra porta publicada)"

	cat > /etc/systemd/system/gestpg-docker-user-forward.service <<'UNIT'
[Unit]
Description=gest-postgres: reaplica DOCKER-USER -> ufw-user-forward apos o Docker subir
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/gestpg-docker-user-forward.sh

[Install]
WantedBy=multi-user.target
UNIT
	systemctl daemon-reload
	systemctl enable gestpg-docker-user-forward.service
	ok "gestpg-docker-user-forward.service habilitado (reaplica o salto sozinho a cada boot)"
fi

# ---------- 7. sobe o stack ----------
# GIT_COMMIT embutido no binário do backend (ldflags, ver Dockerfile) — dá
# pra UI comparar com o HEAD do GitHub e mostrar "atualização disponível".
export GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"

# Modo cloud (CLOUD_MODE=1) não inclui o profile "with-frontend" — o serviço
# frontend nem builda/sobe. Modo local (default) inclui, comportamento de
# sempre intocado.
COMPOSE_PROFILE_ARGS=()
if [[ "$CLOUD_MODE_VALUE" != "1" ]]; then
	COMPOSE_PROFILE_ARGS=(--profile with-frontend)
fi

log "subindo o stack (docker compose up --build -d, commit ${GIT_COMMIT})"
"$DOCKER" compose "${COMPOSE_PROFILE_ARGS[@]}" up --build -d

if [[ "$CLOUD_MODE_VALUE" == "1" ]]; then
	# Migração de servidor já rodando: se o frontend tava de pé de uma
	# instalação anterior (modo local), o `up` acima sem o profile não o
	# derruba sozinho (profile só controla o que SOBE, não o que já tava
	# rodando) — precisa parar explícito.
	log "modo cloud: derrubando frontend local (se estava rodando)"
	"$DOCKER" compose stop frontend >/dev/null 2>&1 || true
	"$DOCKER" compose rm -f frontend >/dev/null 2>&1 || true
	if command -v ufw >/dev/null 2>&1; then
		ufw deny 28080/tcp >/dev/null 2>&1 || true
		ufw deny 4173/tcp >/dev/null 2>&1 || true
		ok "ufw: 28080/tcp e 4173/tcp bloqueadas na internet (defesa em profundidade — porta do backend já é loopback-only, acesso de verdade é só via cloudflared)"
	fi
elif command -v ufw >/dev/null 2>&1; then
	# Modo local (default, ou depois de --cloud-disconnect) — remove regras
	# de bloqueio de uma migração cloud anterior, se existirem. `ufw delete
	# deny` falha se a regra não existir; sempre com `|| true`, nunca crítico.
	ufw delete deny 28080/tcp >/dev/null 2>&1 || true
	ufw delete deny 4173/tcp >/dev/null 2>&1 || true
fi

log "esperando backend responder em /api/v1/healthz"
for i in $(seq 1 30); do
	if curl -fsS http://127.0.0.1:28080/api/v1/healthz >/dev/null 2>&1; then
		ok "backend no ar"
		break
	fi
	[[ $i -eq 30 ]] && warn "backend não respondeu em 30s — checa 'docker compose logs backend'"
	sleep 1
done

echo
ok "setup concluído"
if [[ "$CLOUD_MODE_VALUE" == "1" ]]; then
	echo "  modo:     CLOUD — frontend local desligado, acesso só via Cloudflare Tunnel"
	echo "  backend:  http://127.0.0.1:28080/api/v1/healthz (loopback only)"
	echo "  reverter: sudo ./setup.sh --cloud-disconnect"
else
	echo "  frontend: http://localhost:4173"
	echo "  backend:  http://localhost:28080/api/v1/healthz"
fi
echo "  logs:     docker compose logs -f"
ADMIN_PASSWORD_SET="$(grep -m1 '^ADMIN_PASSWORD=' .env | cut -d= -f2-)"
if [[ -n "$ADMIN_PASSWORD_SET" && "$ADMIN_PASSWORD_SET" != "troque-esta-senha" ]]; then
	echo "  login:    admin / ${ADMIN_PASSWORD_SET}  (guarda essa senha, só aparece aqui)"
else
	warn "ADMIN_PASSWORD não gerada (instalação existente de antes dessa versão) — o backend gera uma sozinha no primeiro boot, checa 'docker compose logs backend | grep -i senha'"
fi
[[ "$REAL_USER" != "root" ]] && echo -e "${c_yellow}!${c_reset} deslogue/logue de novo pra usar 'docker' sem sudo" || true
