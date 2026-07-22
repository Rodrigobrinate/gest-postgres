# Plataforma

## Login e RBAC

Sessão em `admin_sessions`, no banco de metadados — não em memória, sobrevive a restart do backend.

Dois papéis:

- **admin** — acesso total.
- **viewer** — só leitura.

Regra de acesso é uma só: **qualquer método não-GET exige `admin`** — cobre as 150+ rotas da API sem marcar rota por rota. GET fica liberado pra `viewer` por padrão, com exceções explícitas onde um GET devolve segredo ou muda estado — ver [Segurança](seguranca.md#autorização-por-método-http-já-falhou-duas-vezes):

- **Terminal** (WebSocket via GET, mas dá controle total) — admin-only mesmo sendo tecnicamente leitura.
- **Inspecionar/logs/estatísticas de container**, **download de backup**, **listar canal de notificação**, **fluxo OAuth do Google Drive** — todos GET que expunham segredo ou mudavam estado, fechados com `requireAdmin`.
- **Logout** — `viewer` consegue, mesmo sem ser admin (ação sobre a própria sessão, não sobre dado da plataforma).

> Sem RBAC granular por servidor/container individual no MVP — é admin-vs-viewer global. Granular fica pra backlog.

**Sessão**: janela deslizante de 30 dias (uso contínuo nunca expira no meio) com teto absoluto de 90 dias independente de atividade — fecha o caso de um token roubado usado esporadicamente nunca expirar. Throttle de login é por IP (não por username), com backoff exponencial; só confia em `X-Forwarded-For` do peer configurado em `TRUSTED_PROXIES` (vazio por padrão — ver [Instalação](instalacao.md#variáveis-de-ambiente-env)), nunca de header não-verificado.

### Step-up de sessão

Algumas operações sensíveis (escrever/enviar/excluir no [gerenciador de arquivos do host](docker-infra.md#do-host)) exigem reconfirmar a senha (`POST /api/v1/auth/step-up`), que eleva a sessão por 5 minutos.

### Throttle de login

Por **IP** (não por username, pra não deixar um atacante trancar o admin de verdade), com backoff exponencial. Usuário inexistente compara contra um hash dummy — fecha enumeração de usuário por timing.

## Dashboard principal

4 cards estilo EasyPanel — número grande + sparkline colorido embaixo (histórico curto em memória, ~1h a 15s/amostra) pra **CPU / memória / disco / rede**. Valores ao vivo (CPU/memória) ficam vermelhos se subiram e verdes se desceram desde o poll anterior (igual ticker de mercado).

Abaixo, **tabela por container**: CPU, memória, peso do container, I/O de disco, rede.

### Origem dos números

- **Rede / I/O** — soma dos containers Docker (sem acesso ao host além da API Docker pra esses — não tem um jeito simples de medir rede/I/O do host inteiro via `/proc`, e contadores acumulados por container já dão um número honesto).
- **Disco** — número real do **host** (total/usado/livre), via `statfs` num mount read-only de um único arquivo (`/etc/hostname`, não o diretório `/etc` inteiro) dentro do container do backend. "Usado"/"total" seguem a mesma convenção do `df`: a reserva do ext4 pra root (tipicamente 5% do filesystem) não conta nem como usado nem como livre — contá-la como "usado" (bug corrigido depois de comparar com o EasyPanel no mesmo host) inflava o percentual sem uso real nenhum acontecendo.
- **Memória (total e usada)** e **CPU** — número real do **host**, não soma de container. `/proc/meminfo` (mount `/hostmem`) pra memória, `MemAvailable` (não `MemFree` — senão cache de página conta como "uso" e infla o número) pro cálculo de usado. `/proc/stat` (mount `/hostcpu`) pra CPU — precisa de duas leituras pra calcular %, então a primeira chamada depois do backend subir cai pro fallback de soma de container só naquela vez. Motivo de existir: soma de container nunca inclui o que roda fora de cgroup Docker (kernel, `dockerd`, sshd, cron, o `firewall-agent`/`update-agent` que rodam no host de propósito) — corrigido depois de comparar ao vivo com o EasyPanel no mesmo host e ver CPU 0.2% vs 25%, memória 8% vs 39%.

### Peso do container

Soma dos volumes nomeados montados quando existem; senão, tamanho da camada gravável (`SizeRw`, via `docker system df`). **Nunca** `SizeRootFs` — isso conta a imagem base inteira, inflando qualquer container que compartilhe imagem com outro.

### cgroup v1 vs v2

Containers em host cgroup v2 reportam `blkio_stats` com `op` minúsculo (`"read"/"write"`), não maiúsculo como cgroup v1 — sem tratar os dois casos, I/O de disco por container sempre dava zero.

## Verificação de atualização

Botão **Verificar atualização** no header do dashboard — compara o commit rodando com o HEAD do branch `main` no GitHub (API pública, sem token) e mostra se está atualizado, com commit/data/mensagem mais recente. Duas formas de aplicar:

- **Copiar o comando** (`git pull && sudo ./setup.sh`) e rodar por SSH — sempre disponível.
- **Botão "Atualizar agora"** — aplica de verdade, sem sair da UI. Exige [step-up de senha](#login-e-rbac) antes (é a ação de maior blast radius de toda a API: roda comando com privilégio de root no host inteiro). Só aparece se o `update-agent` estiver instalado e rodando no host (droplets numa versão anterior a essa funcionalidade caem de volta só no comando pra copiar).

`GIT_COMMIT` é embutido no binário do backend via ldflags em build time (`internal/version.Commit`), passado como build arg a partir de `git rev-parse --short HEAD` que o `setup.sh` exporta antes do `docker compose up --build`. Build manual (sem passar pelo `setup.sh`) fica com commit `dev` — a UI detecta isso e avisa que não dá pra comparar, em vez de comparar errado.

### Como "Atualizar agora" funciona por baixo

Dispara `POST /api/v1/update/apply`, que fala com o **`update-agent`** — processo Go separado, roda no HOST via systemd (nunca em container, mesmo raciocínio do [`firewall-agent`](arquitetura.md#firewall-agent)), escuta só em `/run/gestpg-update.sock`, bind mount no backend. A pipeline disparada é sempre a mesma fixa — `git pull` no repositório configurado em install time + `./setup.sh` desse repositório — nenhum endpoint aceita comando arbitrário vindo da rede.

A execução em si roda numa **unit systemd transiente separada** (`systemd-run --unit=... --collect`), não como filho direto do `update-agent`. Motivo: o próprio `setup.sh` reinstala e reinicia o `update-agent` a cada execução (pra pegar código novo do agente sem depender de reboot manual) — se a pipeline rodasse como filho direto, `systemctl restart gestpg-update-agent` (chamado pelo `setup.sh` que a própria pipeline está executando) mataria a atualização no meio do caminho, porque o systemd mata todo processo do cgroup do serviço ao reiniciar. Rodando numa unit própria, a pipeline sobrevive ao agente reiniciar (ou até cair) no meio do processo. Estado e log ficam em arquivo em disco, nunca em memória — sobrevivem à mesma troca.

Se o host reiniciar (ou o processo cair) no meio de uma atualização, o próximo `GET /status` detecta que a unit transiente não existe mais e destrava o botão pra tentar de novo, em vez de travar pra sempre em "atualizando".

**Segredo protegido**: `setup.sh` ecoa a senha de admin em texto puro no fim de toda execução bem-sucedida (não só na instalação) — o `update-agent` redige essa linha antes de devolver o log pela API, e limpa os códigos de cor ANSI que `setup.sh` imprime.

**Sem reversão automática**: se `git pull` falhar (repo com mudança local, por exemplo), a pipeline só reporta a falha — nunca roda `git reset --hard`/`git clean -f` sozinha.

## Notificações

Canais de alerta (Telegram/webhook) — ver [Docker genérico → Notificações](docker-infra.md#notificações).

## CORS e cookies

- **CORS é allowlist** (`ALLOWED_ORIGINS`), não reflexão de `Origin` — requisição de uma origem fora da lista falha por CORS.
- **Cookie de sessão** ganha `Secure` dinâmico (só quando a requisição já chega por HTTPS, via `X-Forwarded-Proto` ou TLS direto) — instalação padrão em HTTP puro continua funcionando igual.
