# Relatório de Segurança — gest-postgres (2026-07-22)

Pentest / revisão de segurança da plataforma. Escopo: backend Go (`backend/`),
agentes de host (`firewall-agent/`, `update-agent/`), `setup.sh`,
`docker-compose.yml`, frontend (`frontend/`). O repo do sistema mestre
(`gest-postgres-master`, Worker/Pages) está **fora** deste escopo — mas ver a
nota sobre chave de integração em §Contexto.

Metodologia: leitura direta do código (não só do histórico). Mapa de rotas +
middleware conferido em `router.go`/`middleware.go`; varredura por domínio
(autorização, injeção/traversal, SSRF/cripto/segredos, agentes de host/deploy,
frontend); cada achado HIGH verificado à mão contra o código-fonte antes de
entrar aqui.

## Contexto do modelo de autenticação

Duas portas de entrada em `withAuth`:
1. Cookie de sessão → `Session` com papel `admin` | `viewer`.
2. `Authorization: Bearer <chave de integração>` → sessão de serviço **sempre
   admin** (bypassa cookie/CORS), pro Worker do sistema mestre.

Regra de autorização central: **`viewer` alcança QUALQUER rota GET**; só
não-GET (POST/PUT/PATCH/DELETE) e o sufixo `/exec` exigem admin.
`requireAdmin`/`requireElevated` são opt-in, aplicados rota a rota. `viewer` é o
papel de menor privilégio (monitoramento read-only).

Consequência estrutural: toda rota GET nova que carrega segredo precisa ser
lembrada e fechada à mão. **As três falhas HIGH abaixo são todas irmãs
esquecidas de rotas que auditorias anteriores (Leva 6/8) já fecharam** —
mesma classe, endpoints diferentes.

---

## Sumário

| # | Sev | Área | Achado |
|---|-----|------|--------|
| H1 | ALTO | Authz | `GET /servers/{id}/alert-rules` devolve `webhook_url` (segredo) pra viewer |
| H2 | ALTO | Authz | `GET /infra/volumes/{v}/backups/{id}/download` não é admin-only — viewer baixa o volume inteiro |
| H3 | ALTO | Authz | `GET /infra/git-deployments` devolve `env` (senhas em texto puro) pra viewer |
| H4 | ALTO | Exposição | API com poder de root-no-host publicada em `0.0.0.0` sobre HTTP puro por padrão |
| M1 | MÉDIO | Authz | Gerenciador de arquivos do host: leitura/download alcançável por viewer (escrita pede step-up) |
| M2 | MÉDIO | Supply-chain | `docker-socket-proxy` (imagem mais privilegiada) em `:latest`, não pinado |
| M3 | MÉDIO | Deploy | Update roda `./setup.sh` como root de um checkout dono do usuário não-root |
| M4 | MÉDIO | Cripto | `SecretBox` sem AAD — 7 tipos de segredo numa chave só (confused-deputy) |
| M5 | MÉDIO | Frontend | CSP `script-src 'unsafe-inline'` (trade-off do static export) |
| L1 | BAIXO | Authz | `GET /infra/cron-jobs` devolve `command` + `last_output` pra viewer |
| L2 | BAIXO | Segredos | Erro de webhook genérico vaza basic-auth embutido na URL |
| L3 | BAIXO | SSRF | `git clone` valida host só no save/exec-time (DNS-rebinding TOCTOU), admin-only |
| L4 | BAIXO | Segredos | `ADMIN_PASSWORD` em texto puro em `/var/log/gestpg-update.log` |
| L5 | BAIXO | Robustez | `/update/apply` concorrente colide em nome de unit no mesmo segundo |
| L6 | BAIXO | Robustez | Resposta de `api.ipify.org` entra no `.env` via `sed` sem validação |
| L7 | BAIXO | Firewall | Jump `DOCKER-USER→ufw` não reaplicado em `systemctl restart docker` (só boot) |
| L8 | BAIXO | Frontend | `id` de instalação/servidor sem `encodeURIComponent` (self-controlled) |
| I1 | INFO | Injeção | Filtro `;`-only em campos de SQL cru admin-only (cosmético) |

---

## Achados detalhados

### H1 — ALTO — `ListAlertRules` vaza `webhook_url` pra viewer
`backend/internal/server/alerts.go:75` · rota `router.go:294`

`GET /api/v1/servers/{id}/alert-rules` **não tem `requireAdmin`**.
`ListAlertRules` seleciona `webhook_url` e devolve em `AlertRule.WebhookURL`
(`json:"webhook_url,omitempty"`, alerts.go:15,88). Pra Slack/Discord/webhook
genérico, o path da URL **é** a credencial. É exatamente o segredo que fez a
lista de `notification-channels` virar admin-only na Leva 8 (`router.go:120`) —
mas a regra de alerta que guarda uma `webhook_url` **direta** (alerts.go:44)
nunca foi fechada.

- **Exploração**: viewer loga → `GET /api/v1/servers/<id>/alert-rules` → lê a
  `webhook_url` de cada regra → posta na endpoint (Slack/Discord/webhook) como
  se fosse a plataforma.
- **Correção**: `requireAdmin(detail.ListAlertRules)` no `router.go:294`
  (paridade com notification-channels). Alternativa: não devolver `webhook_url`
  na lista, só um booleano "tem webhook direto".

### H2 — ALTO — Download de backup de volume não é admin-only
`backend/internal/api/volume_backups.go:38` · rota `router.go:196` (e `194`)

`GET /api/v1/infra/volumes/{volumeName}/backups/{backupId}/download` é GET
**sem gate**. O handler abre e faz `http.ServeContent` do `.tar.gz` do volume
inteiro (volume_backups.go:44-56). A Leva 8 fechou explicitamente o download de
backup de Postgres (`router.go:108`, com a nota "extrapola o que GET=viewer
deveria cobrir") — um snapshot de volume é **estritamente mais amplo**: pode
conter diretório de dados de Postgres, `.env` de aplicações, chaves TLS
privadas, etc.

- **Exploração**: viewer → `GET .../volumes/<v>/backups` (também sem gate,
  `router.go:194`) pra enumerar `backupId` → `.../{backupId}/download` →
  recebe os bytes do volume inteiro.
- **Nota menor**: `DownloadVolumeBackup(ctx, backupId)` ignora o segmento
  `{volumeName}` e usa só o `backupId` — inofensivo hoje, mas o path param é
  decorativo.
- **Correção**: `requireAdmin(volumeBackupsHandler.Download)` no `router.go:196`
  (igual ao download de backup Postgres).

### H3 — ALTO — `ListGitDeployments` vaza `env` (segredos) pra viewer
`backend/internal/infra/git_deployments.go:117` · rota `router.go:236`

`GET /api/v1/infra/git-deployments` é GET sem gate. `scanGitDeployment`
desserializa `env_json` em `GitDeployment.Env` (`json:"env"`, git_deployments.go:19,156)
e serializa isso na resposta. Env de deployment carrega rotineiramente senha de
banco / API key — mesma superfície que fez o `inspect` de container virar
admin-only na Leva 8 (`router.go:177`). (O `webhook_secret` está corretamente
fora do SELECT da lista — só o `env` vaza.)

- **Exploração**: viewer → `GET /api/v1/infra/git-deployments` → lê `env` (e
  `last_build_log`) de cada auto-deploy configurado.
- **Correção**: gate `requireAdmin`, ou redigir os valores de `Env` na lista.

### H4 — ALTO — API com poder de root-no-host publicada em `0.0.0.0`/HTTP por padrão
`docker-compose.yml:150-156` (backend), `231` (frontend)

O backend é a ponta de uma cadeia que equivale a root-no-host
(`docker-socket-proxy` com `POST+CONTAINERS+EXEC+BUILD`, pipeline de
`update/apply`, escrita em arquivo de host, controle de firewall/tunnel). No
modo local (default) ele publica `28080:28080` e o frontend `4173:4173` em
todas as interfaces, e as regras iptables do próprio Docker furam o `ufw` (o
`setup.sh` documenta isso — a cadeia `DOCKER-USER→ufw-user-forward` fica
instalada porém vazia/inerte por padrão). Efeito líquido: a API inteira
capaz de tomar o host fica alcançável da internet, protegida só por senha de
admin + throttle de login, **sem TLS** (o cookie de sessão só ganha `Secure`
atrás de HTTPS).

- **Exploração**: um único vazamento de credencial, ou qualquer bug de authz
  futuro na 28080 (ex: os HIGH acima), e o atacante tem terminal `EXEC` em
  qualquer container, `docker build`, criar container com bind de host, e
  controle de firewall — todos admin-session-only → comprometimento total do
  host a partir da internet pública. (`update/apply` em si não é alcançável
  sem step-up, mas o resto é.)
- **Mitigação existente**: modo cloud (`setup.sh --cloud-token`) rebind pra
  `127.0.0.1` + `ufw deny 28080/4173` + acesso só via Cloudflare Tunnel. É a
  intenção de projeto.
- **Correção recomendada**: tornar o bind em loopback o **default** do backend
  e forçar todo acesso direto por Traefik/Tunnel; ou, no mínimo, avisar de
  forma proeminente no fim do `setup.sh` que a 28080 está aberta pro mundo.
  Nota: `cap_drop: [ALL]` + `no-new-privileges` no backend **não** reduzem esse
  raio de explosão — o poder está no proxy Docker com que ele fala, não nas
  capabilities do próprio backend.

### M1 — MÉDIO — Gerenciador de arquivos do host: leitura por viewer
`backend/internal/api/host_files.go:30-48` · rotas `router.go:220-222,225`

`host-files` List/Stat/Read/Download são GET sem gate. Leem/streamam o
filesystem real do host confinado a `HOST_FILES_ROOT` (default
`/srv/gestpg-files`). O confinamento em si é **sólido** (`resolveHostPath`:
`filepath.Clean` + join + prefix-check + `EvalSymlinks` + re-check + devolve o
path resolvido, fecha TOCTOU — host_files.go:38-70). O problema é
autorização: escrita/upload/exclusão na **mesma** área exigem `requireElevated`
(step-up), mas leitura/download estão abertas pra viewer. Inconsistente com a
Leva 6, que fez a **leitura** de arquivo de container/volume ser admin-only
(`router.go:151,159`) exatamente por isso.

- **Exploração**: viewer → `GET /api/v1/infra/host-files/content?path=/algo.env`
  (ou `/download` de uma pasta inteira em zip) → lê qualquer arquivo que um
  admin colocou na área gerenciada (config, chave, dump).
- **Correção**: `requireAdmin` em host-files List/Stat/Read/Download, paridade
  com o file manager de container/volume.

### M2 — MÉDIO — `docker-socket-proxy` em `:latest`, não pinado
`docker-compose.yml:74`

`tecnativa/docker-socket-proxy:latest` — o **único** componente que dá acesso
ao Docker Engine (= root no host) é a única imagem não pinada.
`cloudflare_tunnel.go:17` e o Traefik são pinados de propósito (comentário de
"disciplina de supply-chain"); o `backend/Dockerfile` pina `alpine:3.21`/
`golang:1.25-alpine`.

- **Exploração**: um `:latest` malicioso/regredido publicado upstream (ou
  MITM/comprometimento de registry) é puxado silenciosamente no próximo
  `docker compose up --build` / reboot e pode ampliar a superfície da API
  Docker exposta — sem mudança de versão visível pro operador.
- **Correção**: pinar por digest ou versão exata (ex: `:0.3.0` ou
  `@sha256:...`). Tratamento parecido recomendado pra `metadata-db:
  postgres:16-alpine` (prioridade menor — interno).

### M3 — MÉDIO — Update roda `setup.sh` como root de checkout dono do usuário
`update-agent/main.go:57` · `setup.sh:169,281`

A pipeline `cd "$REPO_DIR"; git pull && ./setup.sh` roda como root, e o
`git config --global --add safe.directory "$REPO_DIR"` marca explicitamente um
repo que o próprio `setup.sh` trata como não-root (ele faz `chown $REAL_USER`
em `.env`, `go.mod`, `go.sum`). Então `setup.sh` e o resto da árvore costumam
ser dono/graváveis por `REAL_USER`, não root.

- **Exploração**: qualquer principal que consiga escrever no diretório do repo
  (o usuário de instalação, uma conta dele comprometida, um service account
  que compartilhe a pasta) planta um `setup.sh`/arquivo rastreado malicioso; o
  próximo update clicado por admin executa isso como root. Converte *escrita no
  checkout* (não necessariamente sudo) em root garantido, e sobrevive a rotação
  de senha.
- **Correção**: o update-agent verificar que `REPO_DIR`/`setup.sh` são
  root-owned e não graváveis por grupo/outros antes de rodar (recusar senão);
  ou clonar/ownar a árvore de deploy como root. No mínimo, documentar que o
  diretório do repo é uma fronteira de confiança root.

### M4 — MÉDIO — `SecretBox` sem AAD (confused-deputy)
`backend/internal/crypto/secretbox.go:42,57`

`Seal`/`Open` passam `nil` como additional authenticated data. A **mesma**
`SecretBox` (chave única de config) cifra todas as classes de segredo sem
ligar o blob à coluna: senha de servidor, `client_secret`/`refresh_token` do
gdrive, token do bot Telegram, chave SSH/PAT do git, segredo de webhook do git,
token do Cloudflare Tunnel (7 tipos). Como o GCM autentica integridade mas não
*contexto*, um blob válido decifra em qualquer campo.

- **Exploração**: um ator com escrita numa coluna de ciphertext e leitura do
  efeito de outra pode mover blobs entre campos (ex: um `refresh_token`
  capturado no lugar de `password_encrypted`). Requer escrita no banco ou um bug
  de confusão de coluna — impacto real limitado. É a dívida já reconhecida
  (Leva 6 item 31), aqui com o escopo confirmado.
- **Correção**: AAD por coluna: `Seal(plaintext, aad)` chamando com
  `"gdrive.refresh_token"`, `"server.password"`, etc.; recusar decifragem
  quando o AAD (identidade da coluna) não bate.

### M5 — MÉDIO — CSP `script-src 'unsafe-inline'`
`frontend/scripts/gen-headers.mjs:18` · `frontend/public/_headers:2`

`script-src 'self' 'unsafe-inline'`. Se algum sink de injeção de HTML for
introduzido (ou um existente regredir), `'unsafe-inline'` deixa um `<script>`
injetado executar em vez de ser bloqueado. Hoje **não existe sink de injeção**
(mermaid está `securityLevel: strict`, todo dado não-confiável é renderizado
como texto React escapado) — então é só defesa-em-profundidade, mas remove a
rede de segurança. Trade-off documentado (static export não emite nonce
per-request sem edge function).

- **Correção**: pra fechar de verdade, emitir CSP via Cloudflare Pages Function
  com nonce por resposta e trocar pra `script-src 'self' 'nonce-…'` (tirar
  `'unsafe-inline'`). Senão, manter como está mas rastrear.

### L1 — BAIXO — `cron-jobs` List devolve `command`/`last_output` pra viewer
`backend/internal/infra/cron_jobs.go:92` · rota `router.go:229`

GET sem gate; devolve o `Command` (shell) e o `LastOutput` capturado de cada
job. Não é segredo da plataforma, mas comando frequentemente embute credencial
e o output pode ecoar. **Correção**: `requireAdmin` se a config de cron for
considerada sensível.

### L2 — BAIXO — Erro de webhook genérico vaza basic-auth da URL
`backend/internal/server/notification_channels.go:217-223` · `alerts.go:287`

`redactSecretsInError` só tira o padrão `api.telegram.org/bot<token>`. Pra
webhook genérico, `postJSON` devolve o erro de transporte verbatim; um
`*url.Error` do Go embute a URL inteira. `TestNotificationChannel` mostra isso
direto ao cliente HTTP, e `sweepAlertRulesOnce` loga via `slog.Warn`.

- **Exploração**: admin salva `webhook_url = https://user:pass@host/hook`; o
  botão "Testar" e o log do sweep ecoam `user:pass@host`. Self-inflicted,
  admin-only → BAIXO.
- **Correção**: estender `redactSecretsInError` com o regex `://[^/@\s]+@`
  (já existe em `git_build.go:87`) → `://<redacted>@`.

### L3 — BAIXO — `git clone` valida host só no save/exec-time (DNS rebinding)
`backend/internal/infra/git_build.go:54`

`isInternalHost` resolve e valida uma vez; o `git` re-resolve DNS no exec. Um
host que devolve A público durante a validação e interno (ex:
`169.254.169.254`, `metadata-db`) milissegundos depois no clone alcança o
serviço interno. Admin-only, valor baixo (git fala protocolo git, exfil
limitada). **Correção**: resolver uma vez e clonar contra o IP pinado, ou
aceitar como dívida admin-only documentada.

### L4 — BAIXO — `ADMIN_PASSWORD` em texto puro no log de update
`update-agent/main.go:128,148`

A redação (`redact`, 205-215) só é aplicada no tail devolvido por `/status`. O
log em disco (`> "$LOG_PATH"`) contém a linha `login: admin / <senha>` que o
`setup.sh` ecoa em toda execução bem-sucedida. Arquivo é `0600 root:root`
(não alcançável por não-root hoje), mas qualquer disclosure de arquivo
root-legível futuro vaza a senha atual. **Correção**: redigir antes de
escrever em disco (ou o `setup.sh` suprimir o echo quando invocado
não-interativo).

### L5 — BAIXO — `/update/apply` concorrente colide em nome de unit
`update-agent/main.go:123`

`unitName` = `gestpg-update-run-<unix-segundos>.service`. Dois `/apply` no
mesmo segundo geram o mesmo nome; o segundo `systemd-run` falha em conflito de
unit e grava estado `failed` espúrio. Requer dois requests elevados no mesmo
segundo. **Correção**: sufixo nanossegundo/aleatório no nome + lock em volta do
read-check-launch.

### L6 — BAIXO — Resposta de `api.ipify.org` entra no `.env` sem validação
`setup.sh:154-156`

`PUBLIC_IP` de um serviço HTTP externo é substituído em
`PUBLIC_API_URL`/`ALLOWED_ORIGINS` via `sed s#…#…#`, e `PUBLIC_API_URL` é
embutido no bundle do cliente em build time. Muito baixo (TLS + vendor
confiável), mas uma resposta manipulada com `#`, newline ou origem hostil
corromperia o `.env` ou semearia uma origem atacante no CORS. **Correção**:
validar contra regex IPv4/IPv6 antes de usar; cair pro fallback localhost em
mismatch.

### L7 — BAIXO — Jump `DOCKER-USER→ufw` não reaplicado em restart do dockerd
`setup.sh:426-460`

A oneshot é `After=docker.service` e dispara no boot, mas o `dockerd` recria a
cadeia `DOCKER-USER` vazia em **qualquer** restart do daemon (não só reboot),
derrubando o jump silenciosamente até o próximo boot/`setup.sh`.
Defesa-em-profundidade (a cadeia é vazia por padrão). Se um operador depois
adicionar `ufw route deny` esperando filtrar porta publicada de container,
essas regras podem parar de aplicar após `systemctl restart docker`.
**Correção**: reasserção via trigger path/socket ou `PartOf=docker.service`.

### L8 — BAIXO — `id` de instalação/servidor sem `encodeURIComponent`
`frontend/src/lib/api.ts:35` (e endpoints com `${id}`)

`apiPath()` monta `/proxy/${serverId}${path}` e os endpoints por-servidor
interpolam `${id}` cru (nomes de tabela/role já usam `encodeURIComponent`). O
`id` vem de `?installation=`/`localStorage`/`?id=` — self-controlled, mesma
origem, já autenticado → não cruza fronteira de confiança. **Correção**:
`encodeURIComponent` consistente + validar formato UUID (robustez).

### I1 — INFO — Filtro `;`-only em campos de SQL cru admin-only
`backend/internal/server/tables.go:73`, `db_objects.go:329`, `functions.go:85`

`col.Default` (CREATE TABLE), `checkExpr` (CREATE DOMAIN) e `identityArgs`
(DROP FUNCTION) são concatenados crus na DDL e só rejeitados se contiverem `;`
— trivialmente burlável sem `;` (ex: `(SELECT ...)`). **Não é escalação**: todas
são POST → `requireAdmin`, e admin já tem SQL irrestrito pelo editor. Bate com
a decisão documentada no CLAUDE.md. **Correção** (opcional): largar o teatro do
`;` e documentar como campos de SQL-arbitrário-admin.

---

## Verificado e OK (dá confiança)

**Autenticação / sessão**
- Token de sessão e chave de integração: `crypto/rand` 32B, guardados só como
  SHA-256 no banco, validados por lookup (sem `==` de segredo). bcrypt custo
  10; hash dummy comparado em usuário inexistente (timing-safe). Throttle de
  login por IP com backoff exponencial; janela deslizante 30d + teto absoluto
  90d.
- `clientIP`: usa `RemoteAddr` por padrão; só honra `X-Forwarded-For` do peer
  em `TRUSTED_PROXIES` (vazio por padrão) e lê a entrada mais à direita — não
  forjável (throttle não é mais burlável, achado anterior fechado).
- Cookie `Secure` dinâmico (HTTPS), `HttpOnly`, `SameSite=Lax`.
- **Chave de integração NUNCA satisfaz step-up** (`middleware.go:163`, checado
  antes de `Elevated()`) — uma chave vazada do lado Cloudflare **não** dispara
  `update/apply` (root) nem exclusão de arquivo de host. Controle forte, bem
  colocado.

**Autorização (bem fechado)**
- `servers` List/Get: `PasswordEncrypted`/`ContainerID` são `json:"-"` — sem
  vazamento de senha.
- Já admin-only corretamente: `GET /servers/{id}/password`, download de backup
  Postgres, users List, notification-channels List, gdrive auth-url/callback,
  container inspect/logs/stats, leitura/download de arquivo de container/volume.
- Terminal `/exec` admin-only + Origin-checado (defende CSWSH num shell root).
- `GDriveStatus`/`CloudflareTunnelStatus`/`TraefikStatus` não devolvem
  token/segredo. `ListGitCredentials` não seleciona a coluna de segredo.

**Injeção / traversal (sólido)**
- **Nenhuma** SQLi/cmd-injection/traversal explorável no escopo. Todo
  identificador de Postgres passa por `identRegex` (`^[a-zA-Z_][a-zA-Z0-9_]*$`)
  e/ou `pgx.Identifier{}.Sanitize()`; valor de GUC via nome allowlistado +
  literal com `'` escapado; tipo de coluna via allowlist.
- Todo `exec.Command` usa argv separado (sem `sh -c` no host). `git clone`:
  allowlist de esquema, rejeita `::`/`-` inicial, `isInternalHost`,
  `GIT_ALLOW_PROTOCOL`; PAT via `GIT_ASKPASS` (fora do argv). `pg_dump -d`
  valida database (sem injeção de conninfo). `PGPASSWORD` via env, não argv.
- Confinamento de path (host/container/volume/backup): `filepath.Clean` + join +
  prefix-check + `EvalSymlinks`; `validateVolumeName` mata bind de `/` e `:`.
- `firewall-agent`: `validateRule` exige proto tcp/udp, porta 1-65535,
  **bloqueia 22/tcp**, valida `From` como IP/CIDR; sem `enable/disable/reset`.

**SSRF / outbound**
- `safeDialContext` pina o IP resolvido e revalida no **dial** (fecha DNS
  rebinding); `noRedirectClient` desliga redirect. Blocklist cobre loopback,
  link-local (incl. `169.254.169.254`), RFC1918, `fc00::/7`, **CGNAT
  `100.64.0.0/10`**, IPv4-mapped. Telegram/webhook/alerta direto passam todos
  por aí. gdrive/GitHub são hosts fixos (não user-controlled).

**Agentes de host / setup**
- Sockets dos agentes `0660` (não `0666`). Pipeline de update 100%
  parametrizada (nenhum dado de request chega a shell), roda em unit transiente
  destacada (sobrevive a restart do agente), sem `git reset --hard`. Redação de
  senha + ANSI no `/status`. `ufw allow 22/tcp` antes do `enable` (sem lockout).
  Segredos gerados com `openssl rand`; `.env` `chmod 600` idempotente; chave
  placeholder (todo-zero) recusada no boot.
- `metadata-db` (o cofre de segredos) e `docker-socket-proxy` são internos —
  **sem porta publicada**. Socket montado `:ro` no proxy.

**Frontend**
- Sem XSS: única ocorrência de `innerHTML` é o mermaid do ERD com
  `securityLevel: strict` (DOMPurify) + `sanitizeId` allowlist; log do Postgres,
  grid do editor SQL e tail de update renderizam como texto React escapado.
- Sem token/segredo em `localStorage` (só instalação selecionada +
  histórico SQL em `sessionStorage`, limpo em todos os 3 caminhos de logout).
  Auth só por cookie httpOnly. CSP com `object-src 'none'`, `base-uri 'self'`,
  `frame-ancestors 'none'`, `connect-src` escopado, sem `unsafe-eval`.

---

## Riscos estruturais aceitos (reconhecidos, não novos)

- **Proxy Docker = root-no-host**: `POST+CONTAINERS+EXEC+BUILD` deixa criar
  container privilegiado / bind de host → root. Não corrigível filtrando corpo
  de request. Natureza de uma plataforma que dá gestão Docker genérica ao admin.
- **Terminal EXEC**: qualquer admin ganha shell em qualquer container.
  Deliberado, admin-only.
- **Backend roda como root** no container (sem `USER` no Dockerfile). Mitigado
  parcialmente por `cap_drop: [ALL]` + `no-new-privileges`.
- **Auto-update supply-chain**: `git pull && setup.sh` confia no `main` do
  remote sem verificação de assinatura de commit.

---

## Recomendações priorizadas

1. **Fechar H1/H2/H3 agora** — 3 linhas de `requireAdmin` no `router.go`
   (`294`, `196`, `236`). Baixo esforço, alto impacto, e são exatamente a mesma
   classe já corrigida antes.
2. **H4 / M1** — decidir a postura de exposição: bind em loopback por default
   (forçar Traefik/Tunnel) e, no mínimo, fechar host-files read pra admin (M1) e
   avisar no `setup.sh` que a 28080 está aberta.
3. **M2/M3** — pinar `docker-socket-proxy` e tratar o diretório do repo como
   fronteira root no update-agent. Ambos baratos.
4. **Encerrar a classe de fundo, não os sintomas** — os 4 achados de authz
   (H1/H2/H3/M1/L1) são todos "GET novo esqueceu de fechar". Trocar o
   default-permit-GET por um modelo **default-deny por rota** (marcar
   explicitamente as leituras seguras que viewer pode ver) elimina a classe em
   vez de remendar rota a rota a cada leva. É a maior alavanca de segurança do
   backend hoje.
5. **M4/M5 e os LOW** — dívida rastreada; endereçar quando houver leva de
   hardening dedicada.
