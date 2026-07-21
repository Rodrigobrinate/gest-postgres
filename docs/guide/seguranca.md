# Segurança

Resumo do que foi endurecido em duas rodadas de auditoria de segurança e do que continua sendo trade-off consciente. Detalhe item a item de cada mudança específica fica junto da funcionalidade correspondente nesta documentação — aqui é a visão consolidada.

## Modelo de privilégio de fundo (não mudou, decisão consciente)

`docker-socket-proxy` com `POST + CONTAINERS + EXEC + BUILD` equivale, na prática, a container privilegiado — quem controla essas categorias de chamada consegue montar bind do host e chegar a root no host. **Isso é a natureza de uma plataforma que deliberadamente dá gestão Docker genérica pro admin** — não dá pra tornar isso seguro só filtrando o corpo da requisição no proxy atual. Mitigado até onde dá (validação de `VolumeName`, allowlist de esquema no clone Git, etc.), o resto é escopo aceito.

## Autorização por método HTTP já falhou duas vezes

A regra de fundo é uma só: qualquer método não-GET exige admin; GET é liberado pra `viewer` por padrão, com exceções marcadas rota por rota (`requireAdmin`/`requireElevated`). Isso é conveniente (cobre ~150+ rotas sem marcar uma por uma) mas tem um problema estrutural: **toda rota GET nova que devolva segredo ou mude estado precisa ser lembrada individualmente**, e isso já escapou duas vezes em auditorias diferentes — a primeira leva de correções fechou "`viewer` lê `/proc/1/environ`" só na rota do gerenciador de arquivos; a rota irmã de inspecionar container (mesmo segredo, `Config.Env` do container) ficou aberta até a segunda auditoria achar. Ver [segunda rodada](#segunda-rodada-de-auditoria-2026-07-21) abaixo pro detalhe.

Não trocado por um modelo de autorização explícito por capacidade (negar por padrão, liberar só o que for comprovadamente seguro) — seria a correção estrutural de verdade, mas é um refactor maior que ficou fora de escopo das correções pontuais feitas até agora. Registrado aqui como dívida conhecida, não esquecida.

## Maior blast radius de toda a API: "Atualizar agora"

O botão de [aplicar atualização](plataforma.md#como-atualizar-agora-funciona-por-baixo) (`POST /api/v1/update/apply`) roda `git pull && ./setup.sh` com privilégio de root no host inteiro — não só num container. É estritamente mais sensível que qualquer outra ação da plataforma (mesmo o terminal web mira um container específico; isso aqui mira o host todo, incondicionalmente). Reverte uma decisão de arquitetura anterior ("nunca aplica nada sozinho") — confirmado explicitamente com o usuário antes de implementar, aceitando o risco.

Mitigações:

- Exige [step-up de senha](plataforma.md#login-e-rbac) (sessão elevada), não só admin — mesma trava do gerenciador de arquivos do host, aplicada aqui por ser a ação de maior risco.
- Nenhum dado da requisição chega perto de um shell — a pipeline é uma string fixa no binário do `update-agent`, sem parâmetro nenhum vindo da API.
- Sem reversão automática — falha de `git pull`/`setup.sh` só é reportada, nunca "consertada" com `git reset --hard`/`clean -f`.
- Log da execução redige a senha de admin que `setup.sh` ecoa no fim de toda execução bem-sucedida, antes de expor via API.

Risco residual aceito conscientemente: um atacante que comprometa uma sessão de admin **e** consiga o step-up (senha) ganha equivalente a acesso root ao host via um clone de repositório Git controlado por terceiro (supply chain do próprio `github.com/Rodrigobrinate/gest-postgres`) — mesma classe de risco que qualquer `curl | sudo bash` de instalação, não introduzida por essa feature especificamente, mas agora acionável sem SSH.

## O que foi corrigido (resumo)

| Área | Mudança |
|---|---|
| CORS | Allowlist (`ALLOWED_ORIGINS`) em vez de reflexão de `Origin` |
| Cookie de sessão | `Secure` dinâmico via `X-Forwarded-Proto` |
| Arquivo de container/volume | Leitura virou admin-only (antes GET liberado pro `viewer` — fechava o caminho de ler `/proc/1/environ` do próprio backend e roubar `CREDENTIAL_ENCRYPTION_KEY`) |
| Containers da própria plataforma | Fora de alcance do file manager de container; `/proc` bloqueado como caminho em qualquer container |
| `git clone` | Allowlist de esquema, bloqueio de `::` (mata transporte `ext::`, RCE), bloqueio de argumento iniciando com `-`, `GIT_ALLOW_PROTOCOL` — item **crítico/RCE** do relatório |
| Token de PAT Git | Não vai mais no argv (`/proc/<pid>/cmdline`) — passa por `GIT_ASKPASS` |
| `ufw route allow/deny` | Passou a filtrar porta publicada por container de verdade (cadeia `DOCKER-USER` → `ufw-user-forward`) |
| Backend (compose) | `cap_drop: [ALL]` + `no-new-privileges` |
| Mount de disco | `/etc` inteiro → só `/etc/hostname` |
| `.env` | `chmod 600` idempotente em toda execução do `setup.sh` |
| OAuth Google Drive | `state` deixou de ser constante fixa (`"gestpg"`) → aleatório com expiração — fechava CSRF de vinculação de conta |
| Token de bot Telegram | Parou de vazar em log/erro de transporte |
| `webhook_url` | Guarda de SSRF (recusa loopback/link-local/privado), sem seguir redirect |
| `database` (backup) | Validação de identificador — fechava path traversal e injeção de conninfo no `pg_dump -d` |
| `VolumeName` | Validação de formato — sem isso, um valor `"/"` virava bind da raiz do host (Docker trata bind e volume nomeado pelo mesmo campo) |
| Rotas Traefik | Guarda de host interno + rejeição de aspas/quebra de linha em `target_url`/`redirect_target` (SSRF + escape de YAML) |
| `pg_hba.conf` | Rejeita espaço/tab embutido nos campos |
| Histórico do editor SQL | `localStorage` → `sessionStorage`, limpo no logout |
| Frontend | CSP + `X-Frame-Options: DENY` + `X-Content-Type-Options: nosniff` |
| Requisições JSON | Limite de 10MB (`io.LimitReader`) |
| Servidor HTTP | `ReadTimeout`/`IdleTimeout` (sem `WriteTimeout` de propósito — quebraria download grande e o WebSocket do terminal) |
| Login | Throttle por IP + backoff exponencial + comparação contra hash dummy (fecha enumeração por timing) |
| Webhook Git | Rota casa padrão exato, não sufixo |
| `resolveHostPath` | Devolve caminho já resolvido (pós-symlink) — fechava um TOCTOU |
| Nome de arquivo | Rejeita `.`/`..` |

## Segunda rodada de auditoria (2026-07-21)

Verificação independente da primeira remediação contra o código atual — 17 achados (1 crítico, 3 altos, 6 médios, 7 baixos), todos fechados.

| Severidade | Achado | Correção |
|---|---|---|
| Crítico | `viewer` lia `CREDENTIAL_ENCRYPTION_KEY`/`ADMIN_PASSWORD` via `GET .../containers/{id}/inspect` (env do container do backend, sem gate) | `requireAdmin` em `inspect`/`logs`/`stats` |
| Alto | Throttle de login burlável forjando `X-Forwarded-For` (porta publicada crua, sem proxy real na frente pra confiar no header) | `TRUSTED_PROXIES` — vazio por padrão, header só é honrado do peer imediato configurado explicitamente |
| Alto | `viewer` sequestrava o destino de backup do Google Drive via `GET /gdrive/auth-url` + `/callback` (GET que escreve estado persistente) | `requireAdmin` nos dois |
| Alto | Mesmo vazamento do crítico, nas rotas irmãs de log/stats de container | Coberto junto com o crítico acima |
| Médio | `CREDENTIAL_ENCRYPTION_KEY` toda-zero (placeholder do `.env.example`) passava pela validação | Boot recusa explicitamente esse valor conhecido-público |
| Médio | Salto `DOCKER-USER → ufw-user-forward` não sobrevivia a reboot, só IPv4 | Unit systemd oneshot reaplica após `docker.service`; cobre `ip6tables` |
| Médio | Socket do `firewall-agent`/`update-agent` era `0666` — qualquer usuário local do host manipulava regra de firewall ou disparava update como root | `0660` (os agentes já rodam como root, suficiente) |
| Médio | CSP com `unsafe-eval` (desnecessário em produção) | Removido, testado ao vivo sem quebra |
| Médio | Download de `pg_dump` completo e listagem de `webhook_url` (segredo-portador) liberados pra `viewer` | `requireAdmin` nos dois |
| Baixo (7 itens) | Symlink em download-zip do gerenciador de arquivos do host; histórico SQL não limpo em logout automático; sessão sem teto de vida absoluto; SSRF sem `100.64.0.0/10` (CGNAT) e sem dialer pinado contra DNS rebind; webhook do Git com oráculo de ID (404 vs 401) e sem rate-limit; `backupFilePath` de volume sem re-checagem de confinamento; `git clone` aceitava `git://` e não checava host interno | Todos corrigidos — ver [`CLAUDE.md`](https://github.com/Rodrigobrinate/gest-postgres/blob/main/CLAUDE.md) (Leva 8) pro detalhe técnico de cada um |

`TRUSTED_PROXIES` é a peça nova mais visível — variável de ambiente vazia por padrão (ver [Instalação](instalacao.md#variáveis-de-ambiente-env)); só popular se um reverse proxy de verdade estiver na frente do backend.

## Debt reconhecido

- **Rotação/AAD de `CREDENTIAL_ENCRYPTION_KEY`** — item do relatório original ainda em aberto, já classificado como futuro desde a auditoria.
- **Filtro de SQL só-por-`;`** em `DropFunction`/CHECK de domínio/DEFAULT de coluna — admin-only (que já roda SQL cru pelo Editor SQL), o próprio relatório classificou como "não cruza fronteira de privilégio", cosmético.
- **Modelo de autorização por método HTTP** — ver seção acima; correção estrutural (autorização explícita por capacidade) fica pra uma leva futura.
- **Backend roda como root** dentro do próprio container (sem migração de usuário coordenada com ownership de `HOST_FILES_ROOT`/volumes/socket do `firewall-agent`) — `cap_drop:[ALL]`+`no-new-privileges` já aplicado, mas não é a mesma coisa que não ser root.

## Autenticação de webhook (nota específica)

O endpoint de deploy automático via webhook (`POST .../git-deployments/{id}/webhook`, ver [Docker genérico](docker-infra.md#deploy-automático-via-webhook)) é a **única rota pública** de toda a API — GitHub/GitLab não têm como mandar cookie de sessão. Autenticado só pela assinatura do provedor (HMAC do GitHub, token do GitLab). Padrão de mercado pra esse caso, mas vale saber que é a exceção à regra "tudo exige sessão".

## Antes de expor a instalação na internet

- Configure domínio + HTTPS via [Traefik](docker-infra.md#traefik-reverse-proxy--domínio--ssl) em vez de expor `4173`/`28080` direto.
- Ajuste `ALLOWED_ORIGINS` pro domínio real.
- Considere restringir `28080`/`4173` via [firewall](docker-infra.md#firewall-do-host-ufw) depois que o domínio+Traefik estiver na frente (o `setup.sh` não bloqueia essas portas sozinho, de propósito — garantir acesso direto por IP:porta funcionando "de cara").
