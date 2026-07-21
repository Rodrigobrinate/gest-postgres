# Relatório de Segurança — gest-postgres (2ª rodada / verificação pós-remediação)

**Data:** 2026-07-21
**Autor:** Revisão de segurança (SOC / AppSec)
**Escopo:** toda a base — backend Go (~16k LOC), frontend Next.js, `firewall-agent`, `docker-compose.yml`, `setup.sh`, Dockerfiles, migrations.
**Método:** leitura manual do núcleo de auth/RBAC/cripto/roteamento + 8 auditorias paralelas por domínio (RBAC/sessão/CORS, injeção de comando/RCE, SSRF/OAuth/webhook, path traversal/file manager, cripto/segredos, modelo de privilégio Docker/infra, SQLi, frontend). Cada achado de severidade alta+ foi confirmado por leitura direta do código e por 2+ auditores independentes. Nenhum arquivo foi alterado.
**Relação com a rodada anterior:** este relatório **verifica** o `RELATORIO_SEGURANCA.md` (2026-07-20, 33 itens) contra o código atual e procura regressões / achados novos introduzidos pelas próprias correções.

---

## 0. Resumo executivo

A remediação da Leva 6 foi, no detalhe, **sólida**: dos 33 itens do relatório anterior, a grande maioria está genuinamente corrigida e verificada de forma independente (OAuth `state`, redação de token Telegram, guarda de SSRF, allowlist do `git clone`/`ext::`, validação de `VolumeName`/`database`, CORS por allowlist, WebSocket com checagem de Origin, step-up no file manager, hardening de tar/zip-slip, etc.). Os primitivos de base continuam corretos: AES-256-GCM bem construído, `bcrypt`, token de sessão de 32 bytes guardado só como hash, HMAC de webhook em tempo constante, disciplina de `pgx.Identifier{}.Sanitize()`. **Não há SQL injection nem XSS explorável por anônimo/viewer** — confirmado por re-auditoria independente.

O problema é que **a causa-raiz das duas falhas CRÍTICAS anteriores nunca foi corrigida — só os sintomas.** As falhas #2 e #3 da rodada 1 (viewer lê segredo via `GET`) foram fechadas **endpoint por endpoint** (`requireAdmin` na rota de arquivo, na rota de senha). O modelo por trás — **"autorização decidida pelo método HTTP: GET = seguro pra viewer"** (`middleware.go:108-114`) — continua de pé. Resultado: rotas `GET` que vazam segredo ou mudam estado, que **não estavam na lista original**, seguem abertas, e uma delas é uma CRÍTICA nova funcionalmente idêntica à #2 que acharam ter fechado.

O caso é ilustrativo: em `router.go:117-121` há um comentário explicando que a leitura de arquivo foi posta atrás de `requireAdmin` **justamente** porque "sem isso um viewer lê `/proc/1/environ` de qualquer container (inclusive o do próprio backend, que carrega `CREDENTIAL_ENCRYPTION_KEY`)". A porta ao lado — `GET .../inspect`, que devolve `Config.Env` do container — ficou **sem gate nenhum**, três linhas abaixo (`router.go:146`). Mesmo segredo, mesma vítima, endpoint diferente.

Três frases resumem o risco atual:

1. **`viewer` volta a ser `admin` (e root no host).** Via `GET .../inspect` um viewer lê `CREDENTIAL_ENCRYPTION_KEY` (decifra todo segredo guardado) e `ADMIN_PASSWORD` (login direto como admin) do env do container do backend. **É a CRÍTICA desta rodada.**
2. **O throttle de login virou enfeite.** A chave do rate-limit é `X-Forwarded-For`, confiável sem proxy na frente — a porta 28080 é publicada crua. Atacante manda um XFF diferente por tentativa → backoff nunca acumula → brute-force ilimitado.
3. **A postura de rede/transporte não mudou.** API admin em HTTP puro em `0.0.0.0:28080`, passando por cima do ufw, cookie de sessão sem `Secure` por padrão. Isso segue como dívida aceita da rodada 1, mas amplia o raio de cada item acima.

### Tabela de severidade — achados novos/abertos desta rodada

| # | Sev | Área | Item | Confirmado por |
|---|-----|------|------|----------------|
| **C1** | **CRÍTICO** | RBAC/Segredo | `viewer` lê `CREDENTIAL_ENCRYPTION_KEY` + `ADMIN_PASSWORD` via `GET .../inspect` (env do container, sem redação, sem gate) | 2 auditores + leitura direta |
| **H1** | **ALTO** | Auth | Throttle de login burlável por spoof de `X-Forwarded-For`; mesmo vício afeta cookie `Secure` e redirect do OAuth | 2 auditores + leitura direta |
| **H2** | **ALTO** | RBAC/OAuth | `viewer` sequestra o destino de backup do Google Drive via fluxo OAuth exposto em `GET` (`auth-url`+`callback` sem `requireAdmin`) | 3 auditores + leitura direta |
| **H3** | **ALTO** | RBAC/Segredo | `viewer` lê log de containers da própria plataforma (`backend`/`metadata-db`) via `GET .../logs` → senha admin de bootstrap / connection string; sem `guardNotOwnContainer` em `logs`/`inspect`/`stats` | 2 auditores |
| **M1** | **MÉDIO** | Cripto | Chave placeholder toda-zero é aceita pela validação de config → deploy sem `setup.sh` roda com chave pública do repositório | 1 auditor + leitura |
| **M2** | **MÉDIO** | Firewall | Salto `DOCKER-USER → ufw-user-forward` não é persistente (some no reboot) e é só IPv4 → `ufw route deny` de porta publicada para de valer silenciosamente | 1 auditor |
| **M3** | **MÉDIO** (local) | Firewall | Socket Unix do `firewall-agent` é `0666` sem autenticação → qualquer usuário local do host manipula regras de ufw (menos 22/tcp) | 1 auditor + leitura |
| **M4** | **MÉDIO** | Frontend | CSP permite `'unsafe-inline'` + `'unsafe-eval'` no `script-src` → anula o CSP como defesa contra XSS | 1 auditor |
| **M5** | **MÉDIO** | RBAC | `viewer` baixa `pg_dump` completo de qualquer banco gerenciado (`GET .../backups/{id}/download`) + pagina linhas cruas de qualquer tabela | 1 auditor |
| **M6** | **MÉDIO** | Segredo | `viewer` lê `webhook_url` (segredo-portador de Slack/Discord) via `GET /notification-channels` | 1 auditor |
| **L1** | BAIXO | Path traversal | Download-zip de pasta do host segue symlink filho → viewer exfiltra alvo do symlink (ex: `/etc/shadow`) se existir na raiz | 1 auditor |
| **L2** | BAIXO | Frontend | Histórico SQL (com senhas em texto puro) não é limpo nos logouts automáticos (401 / falha de `/me`), só no botão | 1 auditor |
| **L3** | BAIXO | Auth | Sessão tem expiração deslizante, sem teto de vida absoluto | 1 auditor |
| **L4** | BAIXO | SSRF | Guarda de SSRF valida no cadastro mas o cliente re-resolve no envio (DNS-rebind/TOCTOU, sem dialer pinado); falta `100.64.0.0/10` | 1 auditor |
| **L5** | BAIXO | Auth/DoS | Webhook de git faz decrypt+DB antes de autenticar e responde 404≠401 (oráculo de ID); sem rate-limit na rota pública | 1 auditor |
| **L6** | BAIXO | Path traversal | `backupFilePath` faz `filepath.Join` sem re-checar confinamento (não explorável hoje, quebra o idioma do resto do código) | 1 auditor |
| **L7** | BAIXO | SSRF | `git clone` sem guarda de host interno + permite `git://` em texto puro (admin-only) | 1 auditor |

Itens de privilégio Docker / transporte HTTP / backend root / chave única sem rotação seguem como **dívida aceita e inalterada** da rodada 1 (§4 abaixo).

---

## 1. CRÍTICO — C1: `viewer` lê a chave-mestra de criptografia e a senha admin via `GET .../inspect`

**Arquivos:** `backend/internal/api/router.go:146` (rota sem gate) · `backend/internal/api/middleware.go:108-114` (regra RBAC) · `backend/internal/docker/inspect.go:39,73` (env sem redação) · `docker-compose.yml:107-117` (env do backend).

A regra de autorização é uma só:

```go
// middleware.go:108-114
if !sess.IsAdmin() && !selfServicePaths[r.URL.Path] {
    isWrite := r.Method != http.MethodGet && r.Method != http.MethodHead
    isTerminal := strings.HasSuffix(r.URL.Path, "/exec")
    if isWrite || isTerminal {
        httpx.WriteError(w, http.StatusForbidden, "essa ação exige papel admin")
        return
    }
}
```

Ou seja: **qualquer `GET` (menos `/exec`) passa para um `viewer`.** A rota de inspect é `GET` e não tem `requireAdmin`:

```go
// router.go:146
mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/inspect", infraHandler.ContainerDetail)
```

E `ContainerDetail` devolve o env cru, sem redação e sem excluir os containers da própria plataforma:

```go
// inspect.go:39   struct serializada pro cliente
Env []string `json:"env"`
// inspect.go:73
detail.Env = info.Config.Env
```

**Caminho de ataque (papel `viewer`, o mais baixo):**
1. `GET /api/v1/infra/containers` → lista todos os containers, acha o backend pelo label `com.docker.compose.service=backend`.
2. `GET /api/v1/infra/containers/<backendId>/inspect` → lê `env`, que por `docker-compose.yml:107-117` contém:
   - `CREDENTIAL_ENCRYPTION_KEY` → **decifra todo segredo guardado**: senha de superusuário de todo Postgres gerenciado, `client_secret`/`refresh_token` do Google Drive, PAT/chave SSH do Git, token do bot Telegram, segredos de webhook.
   - `ADMIN_PASSWORD` → **login direto como admin** (takeover imediato).
   - `METADATA_DATABASE_URL` → senha do banco de metadados. Inspecionar o `metadata-db` ainda dá `POSTGRES_PASSWORD`.

Isso é escalonamento total de privilégio a partir de uma conta "só leitura". **É exatamente o segredo que a correção #2 da rodada 1 deveria proteger** — protegido na rota de file manager (`router.go:123-137`, com comentário explicando o porquê em `:117-121`), reexposto pela rota irmã de inspect. O guard `guardNotOwnContainer`/`ownPlatformServices` existe só em `container_files.go`, não no caminho de inspect (`container_detail.go`).

**Correção:**
1. Envolver a rota em `requireAdmin`: `mux.HandleFunc("GET .../inspect", requireAdmin(infraHandler.ContainerDetail))`.
2. Em `ContainerDetail`, chamar `guardNotOwnContainer(ctx, id)` (recusar containers da plataforma) e **redigir `Env`/`Command`** para não-admins mesmo assim — env de qualquer container é sensível.
3. Aplicar o mesmo aos endpoints irmãos `logs`/`stats` (ver H3).

---

## 2. ALTO

### H1 — Throttle de login burlável por spoof de `X-Forwarded-For`

**Arquivo:** `backend/internal/api/auth.go:54-66` (`clientIP`), consumido em `login.go` (throttle por IP).

```go
// auth.go:54-66
func clientIP(r *http.Request) string {
    if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {   // confiado sem condição
        if i := strings.IndexByte(fwd, ','); i >= 0 {
            return strings.TrimSpace(fwd[:i])
        }
        return strings.TrimSpace(fwd)
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    ...
}
```

O backoff por IP (hardening da Leva 6, prometido pelo item #19) usa `clientIP` como chave, e `clientIP` prefere `X-Forwarded-For`. Na instalação padrão a porta 28080 é publicada crua, **sem proxy confiável na frente**, então o header é 100% controlado pelo atacante: um `X-Forwarded-For: 1.2.3.<n>` diferente por request faz cada tentativa parecer um IP novo → o lockout nunca dispara → **brute-force online sem limite algum**. O `bcrypt` freia o throughput bruto, mas não há lockout — uma senha fraca de `viewer` ou de usuário criado pelo admin (o `ADMIN_PASSWORD` default é forte, os demais não têm garantia) fica quebrável.

O mesmo vício de confiar em header não-verificado atinge:
- `isRequestHTTPS` (`auth.go:26-28`) → decide o flag `Secure` do cookie a partir de `X-Forwarded-Proto`; um atacante pode suprimir `Secure` (só relevante em deploy HTTPS).
- `redirectURL` do callback OAuth (`backup.go`) → mesmo header.

**Correção:** usar `r.RemoteAddr` (peer real do socket) como chave do throttle por padrão; só honrar `X-Forwarded-For`/`X-Forwarded-Proto` quando o peer for um CIDR de proxy confiável configurado (`TRUSTED_PROXIES`), e aí ler a entrada **mais à direita** (o hop do seu proxy), não a mais à esquerda (do atacante).

### H2 — `viewer` sequestra o destino de backup do Google Drive (write via GET)

**Arquivo:** `backend/internal/api/router.go:109-110`; handlers em `api/backup.go` + `server/gdrive_config.go`.

```go
// router.go:109-110  — GET, sem requireAdmin
mux.HandleFunc("GET /api/v1/gdrive/auth-url", gdrive.AuthURL)   // grava oauth_state
mux.HandleFunc("GET /api/v1/gdrive/callback", gdrive.Callback)  // grava refresh_token + account_email
```

O fluxo OAuth **muda estado persistente** mas é servido em `GET`, então cai na brecha do C1: um `viewer` consegue dirigir os dois handlers. A correção do `state` fixo (#8) está boa e é verificada — mas o `state` protege contra CSRF de terceiro, **não** contra um viewer legítimo que gera o próprio `state`. Caminho: viewer chama `auth-url` (gera e grava um `state` válido), completa o consentimento Google com a **conta dele**, e o browser cai em `callback?code=…&state=…`; o `state` bate (foi o viewer que gerou), e o backend grava o `refresh_token` **do atacante** como destino de backup da plataforma. Todo backup subsequente — inclusive as políticas agendadas pelo admin, que dão `pg_dump` de **todo banco gerenciado** — passa a subir pro Drive do atacante. Exfiltração passiva e persistente de todos os backups.

Observação: o próprio código já trata um `GET` que dá poder total como exceção — o WebSocket do terminal (`GET .../exec`) é forçado a admin em `middleware.go:110` (`isTerminal`). Os GETs de gdrive foram esquecidos.

**Correção:** `requireAdmin(gdrive.AuthURL)` e `requireAdmin(gdrive.Callback)`. O callback legítimo do admin carrega o cookie admin (SameSite=Lax, GET top-level) e passa; o cookie de um viewer toma 403.

### H3 — `viewer` lê log de container da própria plataforma → senha admin / connection string

**Arquivo:** `router.go:145` (`GET .../logs`, sem gate nem `guardNotOwnContainer`); mesmos irmãos `inspect` (:146) e `stats` (:147-148).

Pela mesma regra GET=viewer, um "monitorador" lê o log de `backend` e `metadata-db`. Isso importa porque `SeedAdminIfMissing` loga a senha admin gerada no stdout quando `ADMIN_PASSWORD` não está setada (`auth/admin.go:34`), e erros de conexão do pgx podem ecoar connection string. O `guardNotOwnContainer` que bloqueia os containers da plataforma existe no file manager mas **não** em `logs`/`inspect`/`stats`.

**Correção:** `requireAdmin` em `logs`/`inspect`/`stats` (ou pelo menos `guardNotOwnContainer` + redação), paridade com o file manager.

---

## 3. MÉDIO

### M1 — Chave de criptografia placeholder (toda-zero) é aceita pela validação

**Arquivo:** `backend/internal/config/config.go:64-66`; `.env.example:8`.

```go
if len(cfg.CredentialEncryptionKey) != 64 {
    return nil, fmt.Errorf("CREDENTIAL_ENCRYPTION_KEY deve ter 64 caracteres hex (32 bytes)")
}
```

Só o **comprimento** é validado. O `.env.example` traz `CREDENTIAL_ENCRYPTION_KEY=0000…0` (64 zeros) — 64 chars, decodifica pra 32 bytes zero, `aes.NewCipher` aceita. Qualquer deploy que pule a regeneração do `setup.sh` (um `cp .env.example .env` manual pra dev/local, ou um `sed` de regen que falhou em silêncio) sobe com uma **chave AES-256 conhecida por qualquer leitor do repositório**. Todo segredo no banco de metadados vira trivialmente decifrável a partir de um dump do banco. O `setup.sh` só regenera quando enxerga o valor todo-zero; o código não tem guarda.

**Correção:** em `crypto.NewSecretBox`, após `hex.DecodeString`, rejeitar chave toda-zero: `if bytes.Count(key, []byte{0}) == len(key) { return nil, errors.New("chave é o placeholder/zero — gere com openssl rand -hex 32") }`.

### M2 — Salto `DOCKER-USER → ufw-user-forward` não é persistente

**Arquivo:** `setup.sh:267-274`.

```sh
iptables -N DOCKER-USER 2>/dev/null || true
iptables -F DOCKER-USER
iptables -A DOCKER-USER -j ufw-user-forward 2>/dev/null || true
iptables -A DOCKER-USER -j RETURN
```

Esse wiring bruto de iptables (o mecanismo inteiro por trás da correção #5) só é aplicado pelo `setup.sh` e **não é salvo** (sem `iptables-persistent`, fora das regras geridas pelo ufw). No reboot o `dockerd` recria `DOCKER-USER` vazia e **não** re-adiciona o salto. Um operador que depois rode `ufw route deny … 28080` pra trancar uma porta publicada vê esse controle **ser ignorado silenciosamente após o próximo reboot** — enquanto a regra ufw ainda aparece como presente. Também é só IPv4 (sem `ip6tables`).

**Correção:** instalar o salto de forma persistente — systemd oneshot `After=docker.service`, ou hooks em `after.rules`/`after6.rules` do ufw — em vez de um comando único no `setup.sh`.

### M3 — Socket do `firewall-agent` é `0666` sem autenticação (escalonamento local)

**Arquivo:** `firewall-agent/main.go:56`.

```go
if err := os.Chmod(socketPath, 0o666); err != nil { ... }
```

`/run/gestpg-firewall.sock` fica `0666` e o agente não autentica request nenhuma. O comentário no código afirma que "só quem tem o FS do host montado no container alcança" — impreciso: um socket `0666` no `/run` global do host é alcançável por **qualquer processo/usuário local do host**. Qualquer foothold não-privilegiado no host adiciona/remove regras de ufw (tudo menos o 22/tcp travado): `deny 80/tcp`, `deny 443/tcp`, `deny 28080/tcp` pra derrubar a plataforma/Traefik, ou abrir portas arbitrárias. Impacto limitado (droplet single-tenant, 22/tcp protegido), daí MÉDIO.

**Correção:** `0660` + rodar o agente sob um grupo dedicado e pôr o backend nesse grupo (ou checar uid do peer via `SO_PEERCRED`), em vez de `0666`.

### M4 — CSP anula a própria proteção contra XSS

**Arquivo:** `frontend/next.config.ts:21`.

```js
"script-src 'self' 'unsafe-inline' 'unsafe-eval'",
```

O header está presente e aplicado (com `frame-ancestors 'none'`, `object-src 'none'`, `base-uri 'self'`, `form-action 'self'` — bom), mas com `'unsafe-inline'` qualquer `<script>` injetado executa e `'unsafe-eval'` libera `eval`/`new Function`. Isso remove o CSP como segunda linha contra XSS. **Não é explorável hoje** (o app não tem sink de injeção de HTML — tudo renderiza como nó de texto React), daí MÉDIO como erosão de defesa-em-profundidade. Recharts/CodeMirror/xterm rodam sob CSP estrito sem `'unsafe-eval'` — só `style-src 'unsafe-inline'` é de fato necessário.

**Correção:** tirar `'unsafe-eval'`; trocar `'unsafe-inline'` do `script-src` por nonce por request (middleware Next) ou ao menos `'strict-dynamic'`. Manter `'unsafe-inline'` só no `style-src`.

### M5 — `viewer` baixa backup completo e pagina linhas cruas de qualquer tabela

**Arquivo:** `router.go:92` (`GET .../backups/{id}/download`), `router.go:62` (`GET .../tables/{schema}/{table}/rows`).

Pela regra GET=viewer, um papel "só leitura" baixa um `pg_dump` completo de qualquer banco gerenciado e pagina linhas arbitrárias — exfiltrando todo dado (e qualquer segredo guardado dentro dos bancos). Parte disso (ler linha de tabela) é o modelo global admin-vs-viewer **documentado**; o **download de backup** extrapola — é o banco inteiro num arquivo. MÉDIO.

**Correção:** pôr o download de backup atrás de `requireAdmin`; decidir explicitamente se leitura de linha crua é aceitável pra `viewer` e gatear se não for.

### M6 — `webhook_url` de canal de notificação exposto a `viewer`

**Arquivo:** `router.go:101` (`GET /notification-channels`); `server/notification_channels.go` (List devolve `webhook_url`).

A rota de listar canais é `GET`, logo viewer-alcançável, e devolve `webhook_url` em texto puro. Pra Slack/Discord/webhook genérico o **path da URL é o segredo-portador** (`https://discord.com/api/webhooks/<id>/<token>`). Um viewer enumera e exfiltra toda URL-capacidade de alerta. (O token do bot Telegram é corretamente omitido do SELECT — mais estreito que o buraco de file-read da rodada 1, mesma classe.)

**Correção:** `requireAdmin` na listagem, ou omitir/redigir `webhook_url` na resposta de lista.

---

## 4. BAIXO / defesa-em-profundidade

- **L1 — Download-zip de pasta do host segue symlink filho.** `api/host_files.go:134-159`. `filepath.Walk` visita symlink-pra-arquivo como entrada não-dir e `os.Open` segue o alvo, sem re-validar com `EvalSymlinks` — burla o confinamento que a leitura de arquivo único (`resolveHostPath`) aplica. Um symlink pré-existente na raiz apontando pra fora (ex: `→ /etc/shadow`) é exfiltrado via download de pasta. `Download` é `GET` → viewer alcança. Sobe pra MÉDIO se tal symlink existir. **Correção:** no callback do walk, pular entradas não-regulares (`info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()`).
- **L2 — Histórico SQL não limpo nos logouts automáticos.** `frontend/src/lib/api.ts:890` (redirect no 401) e `auth-gate.tsx` (falha de `/me`) **não** chamam `clearAllQueryHistory()` — só o botão de logout chama. Texto SQL com `CREATE ROLE … PASSWORD '…'` sobrevive em `sessionStorage` num tab compartilhado após expiração de sessão. **Correção:** chamar `clearAllQueryHistory()` (e limpar o cache do React Query) nos dois caminhos automáticos.
- **L3 — Sessão sem teto de vida absoluto.** `auth/service.go:198-204`: `ValidateSession` reseta `expires_at = now()+30d` a cada request. Token roubado usado ao menos 1×/30d nunca expira. **Correção:** guardar `created_at` e recusar sessão além de um teto (ex: 30-90d) independente de atividade.
- **L4 — DNS-rebind / TOCTOU na guarda de SSRF.** `server/notification_channels.go`, `infra/traefik.go`. A validação resolve DNS **no cadastro**; o request real (envio/alerta/Traefik) re-resolve com o resolver default (sem dialer pinado). Admin cadastra `evil.com`→IP público, depois re-aponta pra `169.254.169.254`/`metadata-db`. Redirect já está desabilitado (bom); resta o rebind. Admin-only, reconhecido em comentário. Falta também `100.64.0.0/10` (CGNAT) no blocklist. **Correção:** `DialContext` custom que re-checa o `addr` **resolvido** no connect, compartilhado pelos clientes que aceitam URL do usuário.
- **L5 — Webhook de git: trabalho antes de autenticar + oráculo de ID.** `infra/git_deployments.go:73-99`. `GitWebhookSecret(id)` (lookup+decrypt) roda **antes** da verificação de assinatura; 404 pra id inexistente vs 401 pra assinatura inválida = oráculo de validade de deployment (mitigado: id é UUID). Rota pública sem rate-limit. **Correção:** 401 idêntico nos dois casos; incluir a rota no bucket de throttle.
- **L6 — `backupFilePath` sem re-checagem de confinamento.** `infra/volume_backups.go:46-48`: `filepath.Join(genericBackupsDir, volumeName, filename)` sem o `HasPrefix` que o lado Postgres (`resolveBackupPath`) aplica. Não explorável hoje (único escritor valida `volumeName`), mas quebra o idioma. **Correção:** rotear por um helper de confinamento espelhando `resolveBackupPath`.
- **L7 — `git clone` sem guarda de host interno + `git://`.** `infra/git_build.go:33-51`. Fecha o `ext::`/`file::` (crítico, verificado sólido), mas não chama `isInternalHost` e `allowedGitSchemes` inclui `git` (texto puro). Admin-only, não alcançável sem sessão. **Correção:** `isInternalHost(u.Hostname())` + dropar `git://`.

---

## 5. Verificação dos 33 itens da rodada 1

| # | Item (rodada 1) | Status atual | Evidência |
|---|-----|------|-----------|
| 1 | RCE `git clone` `ext::`/arg injection | **CORRIGIDO** | `validateGitRepoURL` (bloqueia `::`, `-` inicial, allowlist de esquema) no choke point único `CloneAndBuild:83`; `--` + `GIT_ALLOW_PROTOCOL` (`git_build.go:185,189`). Enforçado inclusive no webhook. |
| 2 | viewer lê `/proc/1/environ` → chave | **CORRIGIDO na rota de arquivo / CLASSE REGREDIDA via inspect** | Rotas de file manager em `requireAdmin`/`requireElevated` (`router.go:123-137`); mesmo segredo reexposto por `GET .../inspect` → **C1**. |
| 3 | viewer revela senha de superusuário | **CORRIGIDO** | `router.go:51` `requireAdmin(detail.Password)`. |
| 4 | Proxy `POST+CONTAINERS+EXEC+BUILD` = root no host | **ACEITO (por design), inalterado** | `docker-compose.yml:59-77` mantém `EXEC:1`+`BUILD:1`. Inerente ao produto. |
| 5 | Portas publicadas passam por cima do ufw | **PARCIAL** | Wiring `DOCKER-USER→ufw-user-forward` presente mas não-persistente/IPv4-only → **M2**. |
| 6 | HTTP/WS texto puro + cookie sem `Secure` | **PARCIAL / ACEITO** | `Secure` dinâmico OK (`auth.go:34-44`); default segue HTTP puro em `0.0.0.0`, TLS opt-in. |
| 7 | Backend root concentrando tudo | **PARCIAL** | `cap_drop:[ALL]`+`no-new-privileges` no compose; **sem** `USER` não-root no Dockerfile → processo ainda UID 0. |
| 8 | OAuth `state` fixo | **CORRIGIDO** | `gdrive_config.go:131-165` state aleatório, expiração 10min, single-use, `ConstantTimeCompare`, migration 0018. (Rota ainda GET → **H2**, problema ortogonal.) |
| 9 | Token Telegram vaza em erro/log | **CORRIGIDO** | `redactSecretsInError` + `botTokenInURLRegex` (`notification_channels.go:198-238`). |
| 10 | CORS reflete qualquer Origin | **CORRIGIDO** | Allowlist `ALLOWED_ORIGINS`, sem `*` (`middleware.go:34-55`). |
| 11 | WS terminal `InsecureSkipVerify` | **CORRIGIDO** | `OriginPatterns` (`terminal.go:43`); terminal admin-only. |
| 12 | PAT do Git no argv/log | **CORRIGIDO** | `GIT_ASKPASS` + env; `redactCredentialsInLog` (`git_build.go:143-198`). |
| 13 | SSRF `webhook_url` | **CORRIGIDO (rebind residual)** | `validateWebhookURL` bloqueia loopback/link-local/private, redirect off. Rebind no envio → **L4**. |
| 14 | Path traversal `database` no backup | **CORRIGIDO** | `identRegex` antes de usar filename (`backup.go:69`); `resolveBackupPath` re-checa prefixo. |
| 15 | Injeção conninfo `pg_dump -d` | **CORRIGIDO** | `identRegex` em `CreateBackup`/política; `PGPASSWORD` em env. |
| 16 | Escrita/exclusão de arquivo sem step-up | **CORRIGIDO** | PUT/upload/DELETE em `requireElevated` (`router.go:126-137`). |
| 17 | `.env` sem `chmod 600` | **CORRIGIDO (race residual)** | `setup.sh:146` idempotente; janela world-readable entre `cp` e `chmod` → ver L (M1-adjacente). |
| 18 | `/etc` inteiro montado | **CORRIGIDO** | `docker-compose.yml:126` `/etc/hostname:/hostfs:ro`. |
| 19 | Sem rate-limit; enumeração por timing | **PARCIAL** | Enumeração por timing **CORRIGIDA** (dummy hash); throttle **burlável** via XFF → **H1**. |
| 20 | Rota pública por sufixo `/webhook` | **CORRIGIDO** | Regex exata âncorada (`middleware.go:71`). |
| 21 | Sem timeouts HTTP; corpo sem limite | **CORRIGIDO** | `ReadTimeout:5m`/`IdleTimeout`; `MaxBytesReader`. |
| 22 | Filtro só-`;` em DropFunction/CHECK/DEFAULT | **INALTERADO (admin-only, cosmético)** | Presente; não cruza fronteira de privilégio. |
| 23 | `pg_hba` aceita espaço embutido | **CORRIGIDO** | `hba.go:98-107` rejeita `" \t"`. |
| 24 | Histórico SQL em `localStorage` | **PARCIAL** | Migrou pra `sessionStorage`; não limpo nos logouts automáticos → **L2**. |
| 25 | Sem CSP / `X-Frame-Options` | **CORRIGIDO (CSP fraco)** | Headers presentes; `script-src` com `unsafe-inline`/`unsafe-eval` → **M4**. |
| 26 | `resolveHostPath` TOCTOU | **CORRIGIDO** (leitura/download) | Retorna caminho resolvido (`host_files.go:64-69`); resíduo no branch de escrita de path novo. |
| 27 | Extração de tar no restore de volume | **CORRIGIDO / mitigado** | Helper `alpine` efêmero; fonte do tar é server-generated. |
| 28 | `validateFilename` não bloqueia `..`/`.` | **CORRIGIDO** | `container_files.go:67-72` rejeita `.`/`..`. |
| 29 | `volumeName` injeta `:` no bind | **CORRIGIDO** | `volumeNameRegex` em todos os sites de bind (`container_detail.go:17-24` etc). |
| 30 | `target_url` Traefik sem guarda/YAML | **CORRIGIDO** | Allowlist de esquema + `isInternalHost` + rejeita `"`/newline (`traefik.go:290-309`). |
| 31 | Chave única sem AAD/rotação | **INALTERADO (dívida aceita)** | AAD `nil` (`secretbox.go:42`); sem rotação. |
| 32 | `sslmode=disable` no Postgres | **CORRIGIDO** | `sslmode=prefer` (rodada anterior). |
| 33 | Senha admin ecoada no setup/log | **INALTERADO (bootstrap aceito)** | `setup.sh` + `admin.go:34-35`; rotacionar após 1º login. |

**Placar:** ~24 genuinamente corrigidos e verificados · 4 parciais (5, 6, 7, 19, 24 — os que geram achados desta rodada) · 4 dívida aceita inalterada (4, 22, 31, 33) · **1 classe regredida** (2 → C1).

---

## 6. Confirmado OK (re-verificado nesta rodada — não reauditar)

- **Sem SQLi por anônimo/viewer.** Re-auditoria independente confirma: todo identificador via `pgx.Identifier{}.Sanitize()`, todo valor via `$N`/`sqlQuoteLiteral`, ORDER BY por allowlist map, LIMIT/OFFSET como `int`. Únicos GETs que montam SQL (`ListSlowQueries`, `TableRows`) são seguros. Toda entrada de SQL cru (editor, EXPLAIN, CREATE FUNCTION/VIEW) é `POST` → admin-only.
- **Sem XSS.** Zero `dangerouslySetInnerHTML`/`innerHTML`/`eval`/`document.write` no frontend; tudo renderiza como nó de texto React; xterm/CodeMirror alimentados com segurança. Token de sessão nunca em storage JS-acessível (cookie httpOnly).
- **Cripto correta.** AES-256-GCM, nonce aleatório de 12 bytes por `Seal`, prefixado; `Open` com checagem de tamanho. Sem reuso de nonce, sem chave hardcoded (exceto o buraco do placeholder toledo-zero, M1).
- **Auth de base sólida.** `bcrypt` (custo default); token de 32 bytes de `crypto/rand`, guardado só como SHA-256; dummy-hash compare fecha enumeração por timing; step-up (`requireElevated`, TTL 5min) sem bypass; ordenação de middleware correta.
- **HMAC de webhook.** `hmac.Equal`/`subtle.ConstantTimeCompare`; segredo por-deployment de 32 bytes, cifrado, devolvido em texto só na criação; rota pública é match exato âncorado.
- **Injeção de comando.** Todo `exec.Command` revisado: argv discreto, sem `sh -c` com entrada não-validada (exceto cron/exec, admin-only por design). `pg_dump`/`pg_restore`/compose/build/prune com regexes de validação e `--`.
- **File manager confinado.** `validateVolumeName`/`validatePath`/`resolveHostPath`/`resolveBackupPath` cobrem os sinks de bind/path; zip-slip do build com guarda antes de escrever; validação antes de `MkdirAll` (sem a regressão de ordem da rodada 1).
- **`firewall-agent` bem trancado.** `ufw` via argv sem shell; `From` por `ParseIP`/`ParseCIDR`; `Proto` restrito; 22/tcp hard-block cobre `allow` e `from X to any port 22`, em add e delete; sem `enable/disable/reset`. (Exceto perm do socket, M3.)
- **Listagens não vazam segredo cifrado.** git-credentials e notification-channels omitem as colunas `*_encrypted` (mas `webhook_url` em texto vaza pra viewer — M6).

---

## 7. Ordem de correção sugerida

1. **C1 — fechar o vazamento de segredo via `inspect`** (`requireAdmin` + `guardNotOwnContainer` + redação de `Env`). Reabre a CRÍTICA da rodada 1; maior impacto, menor esforço.
2. **Corrigir a CAUSA-RAIZ, não só o endpoint (C1, H2, H3, M5, M6, L1).** Trocar a regra "GET = liberado pra viewer" por autorização explícita por rota/capacidade. Enquanto o gate for o método HTTP, cada `GET` novo que devolva segredo ou mude estado é uma falha esperando acontecer. Fazer uma varredura de **toda** rota `GET` perguntando "isso devolve segredo ou muda estado?" e gatear as que sim.
3. **H1 — throttle por `RemoteAddr`**, honrar `X-Forwarded-For`/`-Proto` só de proxy confiável.
4. **M1 — rejeitar chave toda-zero** no boot.
5. **Rede/firewall (M2, M3)** — persistir o salto DOCKER-USER, `0660` no socket do agente.
6. **Endurecimento (M4, L2-L7)** — CSP sem `unsafe-eval`, limpar histórico no logout automático, teto de sessão, dialer pinado anti-rebind, etc.
7. **Dívida estrutural (aceita, planejar):** TLS por padrão + cookie `Secure` forçado; backend não-root; política de `HostConfig` no create/compose (ou documentar "admin = root no host"); rotação/AAD da chave de criptografia.

---

## 8. Conclusão do SOC

A engenharia de segurança pontual do time é boa — os primitivos estão certos e a Leva 6 fechou de verdade quase toda a lista anterior. O risco hoje **não** é falta de competência técnica; é **falta de um modelo de autorização central**. Autorizar por verbo HTTP é uma decisão de arquitetura que já falhou duas vezes (rodada 1: #2/#3) e falhou de novo agora (C1/H2/H3), porque cada rota nova precisa ser lembrada individualmente. Trocar isso por um gate por-capacidade explícito — negar por padrão, liberar leitura de segredo/mudança de estado só pra `admin` — elimina a classe inteira de uma vez, em vez de jogar whack-a-mole a cada auditoria.

Prioridade zero: **C1** (viewer → chave-mestra → tudo). Está a um `requireAdmin` de distância.
