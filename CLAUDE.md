# CLAUDE.md — gest-postgres

Contexto do projeto pra qualquer sessão futura. Ler antes de trabalhar.

## O que é

Plataforma de gestão de instâncias PostgreSQL em Docker: provisionar servidor via UI, configurar tudo do Postgres visualmente, gerenciar extensões, monitorar métricas (Postgres + SO), backup/restore. Público: iniciante (modo Simples, presets) e DBA (modo Avançado, 100% dos parâmetros).

- Ideia original / justificativa de stack: [IDEIA.md](./IDEIA.md)
- Lista completa de requisitos (produto final, todas as "perfumarias"): [REQUISITOS.md](./REQUISITOS.md)

**Fase atual: MVP.** Foco só na seção "Escopo MVP" abaixo. Não implementar itens de "Backlog pós-MVP" sem pedido explícito do usuário, mesmo que estejam documentados em REQUISITOS.md.

## Stack decidida

- **Backend**: Go. `docker/docker/client` (Docker Engine API), `pgx` (driver Postgres), goroutines pra polling de métricas.
- **Frontend**: React + Next.js + shadcn/ui + TanStack Query + Recharts/Tremor pros gráficos.
- **Tempo real**: WebSocket direto do backend Go (`gorilla/websocket` ou `nhooyr.io/websocket`), sem reinventar no front.
- **Banco de metadados**: Postgres separado (servidores registrados, users da plataforma, histórico de config, auditoria, agendamento de backup) — não confundir com os Postgres gerenciados.
- **Docker socket**: nunca acesso direto. Usar `docker-socket-proxy` (`tecnativa/docker-socket-proxy`) liberando só create/start/stop/inspect.

Ainda não decidido / definir quando chegar lá: estrutura de pastas, ORM/query builder no Go (ou SQL puro com `pgx`), lib de auth, formato de deploy final (compose vs binário + compose gerado).

## Convenção de tracking de requisitos

Todo requisito do MVP abaixo é um checkbox. Ao implementar:
1. Marca `[x]` e risca o item: `- [x] ~~texto~~`.
2. Se implementou parcial, deixa `[ ]` e anota o que falta entre parênteses.
3. Não risca por "escrevi o design" — só quando funciona de ponta a ponta (build/roda).

---

## Escopo MVP

### Servidores (ciclo de vida básico)
- [x] ~~Criar servidor via Docker API: nome, versão do Postgres, usuário/senha, porta, preset de recursos (Pequeno/Médio/Grande)~~
- [x] ~~Volume nomeado por instância + rede Docker isolada~~
- [ ] Listar servidores com status (rodando/parado/erro), versão (falta: conexões atuais — sem monitoramento ainda)
- [x] ~~Start / Stop / Restart~~
- [x] ~~Editar servidor (nome, recursos, porta)~~ — nome e recursos aplicam na hora (`ContainerUpdate` do Docker troca CPU/memória de container rodando sem recriar, sem derrubar conexão); porta é a exceção — Docker não permite trocar binding de porta publicada sem recriar o container, então isso para/remove (sem apagar volume)/recria com a porta nova, breve interrupção
- [x] ~~Excluir servidor (confirmação, opção manter/apagar volume)~~

Verificado ponta a ponta em droplet Debian real (wipe total → clone limpo → `sudo ./setup.sh` → criar/start/stop/restart/excluir servidor pela UI, Postgres realmente aceitando conexão). Ver histórico de commits a partir de `f5a557d` até `ee97d0e`.

**Além do MVP**: auto-descoberta de Postgres rodando em containers Docker (fora do MVP original, mas dentro do que a arquitetura atual consegue — sem acesso ao host além da API Docker, então Postgres nativo fora de container fica fora de alcance). Botão "Procurar servidores" na tela inicial lista containers que parecem Postgres e ainda não estão cadastrados; "Cadastrar" pede credenciais reais e só salva depois de confirmar conexão de verdade (`docker network connect` na rede gerenciada se precisar, depois um `SELECT` real) — nada fica registrado com senha errada. Não cria container/volume novo, só passa a gerenciar o que já existe.

### Configuração do Postgres (subset essencial, não tudo de REQUISITOS.md §4)
- [x] ~~Form com os parâmetros mais impactantes: `max_connections`, `shared_buffers`, `work_mem`, `maintenance_work_mem`, `effective_cache_size`, `log_min_duration_statement`~~
- [x] ~~Presets calculam esses valores automaticamente a partir do preset de recursos~~
- [x] ~~Aplicar mudança → reload ou avisa que precisa restart~~
- [x] ~~`pg_hba.conf` básico: tabela simples de regras (tipo, database, user, CIDR, método)~~, sem drag-and-drop ainda — aba Configuração. Lê/escreve o arquivo de dentro do container via API de archive do Docker (`docker cp` por baixo, GET/PUT `/containers/{id}/archive` — não é exec, superfície de ataque bem menor), recarrega via `pg_reload_conf()` sem restart. Regra nova sempre vai pro final do arquivo (não esconde regra mais restritiva já existente); sintaxe inválida faz o Postgres logar erro e manter as regras antigas em memória, não derruba conexão nem trava o servidor

Achado e corrigido um bug de arquitetura sério aqui: a config inicial entrava como flag `-c` no comando do container, que tem prioridade MAIOR que `ALTER SYSTEM` — nenhuma edição pós-criação nunca ia pegar, nem com restart. Agora tudo (inicial e edições) passa por `ALTER SYSTEM` + reload/restart, mesmo caminho.

Configuração expandida bem além do subset original: ~86 parâmetros geridos, agrupados por categoria (memória, conexões, WAL, autovacuum, logging, etc.), com busca, indicação de quais precisam restart vs. reload, e edição só dos campos alterados. Fora do editável de propósito: `listen_addresses`/`port`/`unix_socket_directories`/certificados/`shared_preload_libraries`/`recovery_target_*`/`restore_command` e toggles de debug (`enable_*`) — mudar isso quebra ou exige orquestração que a plataforma ainda não faz.

### Banco de dados / objetos (mínimo pra ser usável)
- [x] ~~Criar/listar/excluir database~~ (aba Monitoramento, card "Bancos de dados" — excluir usa `DROP DATABASE ... WITH (FORCE)`, Postgres 13+, derruba conexões abertas em vez de falhar; banco principal do servidor não pode ser excluído)
- [x] ~~Criar/listar/excluir tabela via formulário~~ (excluir: botão aparece ao passar o mouse na lista de tabelas)
- [x] ~~Editor SQL básico (rodar query, ver resultado em grid, sem autocomplete ainda)~~ — ganhou syntax highlighting (CodeMirror) e histórico de queries também, além do MVP original
- [x] ~~Ver dados da tabela em grid com paginação~~

### Extensões
- [x] ~~Listar `pg_available_extensions`~~
- [x] ~~Habilitar/desabilitar: `pg_stat_statements`, `uuid-ossp`, `pgcrypto`, `pg_trgm`~~ (e qualquer outra da lista, não só essas 4)

`postgres:X` oficial não vem com `pgvector` nem `pg_cron` compilados — servidores novos agora sobem em cima de `gestpg-postgres:X` (`postgres-image/Dockerfile`), a mesma imagem oficial + esses dois pacotes via apt (repo PGDG que a própria imagem já tem configurado). Buildada localmente pelo `setup.sh` uma vez por versão suportada (13-17) — o backend só faz pull/inspect na criação de servidor, nunca build (permissão do docker-socket-proxy é só isso de propósito). `pgvector` funciona na hora (`CREATE EXTENSION vector`, sem restart). `pg_cron` precisa de `shared_preload_libraries` + `cron.database_name`, mesmo tratamento que já existia pra `pg_stat_statements` — clique em "Habilitar" na aba Extensões cuida disso sozinho (reinicia o container, demora mais, badge "requer restart" avisa antes).

Achado e corrigido um bug sério do próprio Postgres nesse processo: `ALTER SYSTEM SET shared_preload_libraries = 'lib1,lib2'` (uma string só com vírgula dentro) faz o Postgres persistir errado no `postgresql.auto.conf` — grava `= '"lib1,lib2"'` com aspas duplas extras envolvendo tudo, e na subida seguinte ele tenta abrir UM arquivo de lib chamado literalmente "lib1,lib2" e trava em crash loop pra sempre. A sintaxe correta é multi-valor (`ALTER SYSTEM SET shared_preload_libraries = lib1, lib2` — identificadores soltos, sem string por fora). Servidores já existentes com só 1 lib preload nunca bateram nesse bug (só aparece com 2+).

Servidores criados antes dessa mudança continuam na imagem `postgres:X` antiga — sem pgvector/pg_cron disponíveis até serem recriados. Sem migração automática no MVP.

### Connection pooling (PgBouncer)
- [x] ~~Connection pooling gerenciado~~ — toggle "Habilitar"/"Desabilitar" na aba Configuração de cada servidor (card abaixo do form de parâmetros)

Sobe um container `edoburu/pgbouncer` companheiro por servidor (nome `{container}-pgbouncer`, sem volume — é stateless, a imagem gera `pgbouncer.ini`/`userlist.txt` sozinha a partir de env vars no boot), numa porta publicada própria alocada da mesma faixa dos servidores. Cliente troca só a porta (mesmo usuário/senha/banco) e passa a falar com o pooler em vez do Postgres direto. Modo de pool escolhível (`transaction` default, `session`, `statement`).

Achado no processo: `AUTH_TYPE=md5` (o default mais comum em tutoriais de pgbouncer) quebra contra qualquer role criada com o `password_encryption` default do Postgres desde a v10+ (`scram-sha-256`) — erro "wrong password type" tanto pro lado cliente→pgbouncer quanto pgbouncer→Postgres. Usa `AUTH_TYPE=scram-sha-256` de propósito.

Excluir o servidor remove o pooler junto (best-effort, não trava a exclusão do Postgres se falhar). Rotacionar a senha do superuser recria o container do pooler com a senha nova — sem isso o pooler ficaria autenticando com credencial velha silenciosamente, mesma classe do bug de rotação de senha já corrigido antes (ver histórico de commits). Trocar a porta do Postgres (editar servidor) não precisa recriar o pooler — ele fala com o Postgres pelo nome do container, não pela porta publicada.

### Monitoramento
- [x] ~~`pg_stat_activity` ao vivo (sessões, query atual, estado), botão cancelar/terminar sessão~~
- [x] ~~Dashboard com gráfico de conexões ao longo do tempo~~ (+ gráfico de CPU/memória — histórico em memória, reseta se o backend reiniciar)
- [x] ~~Top queries lentas via `pg_stat_statements`~~ (aba "Desempenho" — ordenação por tempo total/médio/chamadas, reset de stats, fluxo guiado de habilitação quando a extensão não tá coletando ainda)
- [ ] CPU/RAM/disco por container (docker stats) (falta disco — CPU/RAM ok)

### Logs
- [x] ~~Visualizador de log do Postgres (tail básico, sem parsing estruturado ainda)~~

Tudo isso vive em `/servers/{id}` (clica no nome do servidor na lista) — abas: Monitoramento, Logs, Editor SQL, Tabelas, Extensões, Configuração, Usuários, Desempenho, Objetos, Funções. Backend conecta direto no Postgres gerenciado pela rede `gestpg-managed` (nome do container, não host_port). Verificado ponta a ponta no mesmo droplet a cada feature.

**Além do MVP original**, também saiu nessa leva (pedido explícito do usuário, fora da lista original mas dentro do espírito "gerenciar o banco"):
- Connection string com senha revelável (copiar pra conectar de fora — psql, DBeaver, etc)
- Criar tabela via formulário visual (nome, colunas, tipos, PK, not null, default)
- Gerenciar triggers por tabela (criar/habilitar/desabilitar/excluir)
- Usuários/roles: criar/excluir role, flags (login/superuser/createdb/createrole), matriz de permissões (GRANT/REVOKE SELECT/INSERT/UPDATE/DELETE por tabela)
- Aba "Objetos": Views (criar/listar/excluir), Materialized Views (criar/listar/refresh/excluir), Sequences (criar/listar/excluir), Types/Domains (enum e domain com CHECK, criar/listar/excluir)
- Aba "Funções": functions e procedures — listar (com definição expansível via `pg_get_functiondef`), criar via SQL cru num editor CodeMirror, excluir (suporta overload via assinatura completa)
- Aba "Desempenho": queries lentas via `pg_stat_statements`, com auto-preload da extensão em `shared_preload_libraries` na criação de servidores novos (senão a extensão fica instalada mas não coleta nada) e fluxo guiado de habilitação pra servidores já existentes
- Monitoramento ganhou: lista de databases com tamanho, gráfico de conexões, CPU e memória ao longo do tempo (histórico em memória via goroutine de coleta a cada 15s, reseta se o backend reiniciar)
- Configuração expandida de ~6 pra ~86 parâmetros geridos (ver detalhe acima)

Todos os itens acima testados via curl direto no droplet (criar/listar/refresh/excluir de cada tipo de objeto) e limpos depois. Ver histórico de commits recentes pro detalhe de cada leva.

### Backup / Restore
- [x] ~~Backup manual sob demanda (`pg_dump`, formato custom)~~ — aba "Backup" de cada servidor
- [x] ~~Rotina agendada simples (cron básico: diário/semanal, horário)~~
- [x] ~~Storage local~~ **e Google Drive** (saiu do MVP original, pedido explícito do usuário fora de ordem)
- [x] ~~Restore de backup (sobrescrever servidor original ou criar novo)~~
- [x] ~~Retenção simples: manter últimos N backups~~

`pg_dump`/`pg_restore` rodam DIRETO de dentro do container do backend (não via `docker exec` no container gerenciado — mesma decisão de segurança do resto do projeto, o docker-socket-proxy nunca libera `EXEC`). O binário do cliente é a versão 17 (Alpine 3.21, bump de 3.19 porque esse pacote só existe a partir daí) — cobre dump de servidores v13-17 porque `pg_dump` só consegue falar com Postgres da mesma versão ou mais velho, nunca mais novo. Conecta pelo nome do container na rede `gestpg-managed`, igual toda outra operação do backend.

Arquivo local fica num volume Docker próprio (`backups_data`, montado em `/backups` no backend) — nunca bind mount do host. Backup pro Google Drive não guarda cópia local nenhuma: o dump escreve num arquivo temporário (`/backups/tmp`), sobe em streaming (multipart via `io.Pipe`, nunca carrega o arquivo inteiro em memória — dumps grandes podem ser vários GB) pra uma pasta própria (`gest-postgres-backups`) na conta configurada, e o temporário é apagado. Restore faz o caminho inverso: baixa (do Drive) ou abre direto (local) antes de rodar `pg_restore --clean --if-exists`.

Restore tem dois modos: sobrescrever um banco já existente (apaga tudo que tinha antes) ou criar um banco novo do zero e restaurar nele, sem tocar no original.

"Cron básico" é literal — sem parser de expressão cron de verdade, só frequência (diária/semanal + dia da semana) e horário (UTC), checado a cada 1 minuto contra o último `last_run_at` de cada política habilitada. Retenção (`retention_count`) só conta backups gerados POR AQUELA política — um backup manual nunca é apagado automaticamente por nenhuma política.

Google Drive: cada instalação usa o próprio app OAuth do dono (client_id/secret cadastrado nas configurações — a plataforma não embute nenhuma credencial Google própria). Fluxo padrão: gera URL de consentimento (`access_type=offline&prompt=consent`, garante que a Google devolve `refresh_token` de verdade, não só um `access_token` que expira em 1h), usuário autoriza no navegador dele mesmo (não tem como isso ser automatizado — é a própria Google pedindo login da conta), callback troca o code pelo token e guarda o `refresh_token` cifrado. Implementado com `golang.org/x/oauth2` + chamadas HTTP diretas pra Drive API v3 REST (upload/download/delete), de propósito sem o SDK `google.golang.org/api` — evita puxar uma árvore de dependência bem maior pra só 3 operações.

Testado ponta a ponta no droplet: backup manual local (dump → completo → download → restore em banco novo → restore sobrescrevendo banco existente → delete), e política agendada rodada 3x manualmente com `retention_count=2` confirmando que só os 2 backups mais recentes DAQUELA política sobrevivem (arquivo em disco e registro no banco de metadados, os dois). Integração com Google Drive testada só até a geração da URL de consentimento e o roundtrip da API — a autorização de verdade depende de credenciais OAuth reais que só o dono da conta Google consegue gerar/conceder.

### Plataforma
- [ ] Login/senha — implementado, agora com multi-usuário e 2 papéis (`admin`/`viewer`, ver "Leva 3"; MVP original pedia só 1 usuário admin, saiu maior por pedido explícito), falta verificar ponta a ponta num droplet real antes de riscar
- [x] ~~Dashboard principal com cards de resumo + gráficos do item Monitoramento~~ — 4 cards estilo EasyPanel (número grande + sparkline colorido embaixo, histórico curto em memória ~1h a 15s/amostra) pra CPU/memória/disco/rede, + tabela por container com CPU/memória/peso do container/I/O de disco/rede, valores ao vivo (CPU/memória) ficam vermelho se subiram e verde se desceram desde o poll anterior (igual ticker de mercado). CPU/memória/rede/I/O continuam sendo soma dos containers Docker (sem acesso ao host além da API Docker pra esses); **disco é exceção** — número real do host (total/usado/livre), via `statfs` num mount read-only só de `/etc` (não a raiz inteira, de propósito — é o bastante pra medir o mesmo filesystem que `/` num droplet de disco único sem expor a árvore toda) dentro do container do backend. Rede agora é taxa de verdade (bytes/s, calculada por delta entre amostras do histórico), não mais só acumulado.

Achado nessa leva: containers em host cgroup v2 reportam `blkio_stats` com `op` minúsculo (`"read"/"write"`), não maiúsculo (`"Read"/"Write"` como cgroup v1) — sem tratar os dois casos, I/O de disco por container sempre dava zero.

"Peso do container" generalizado pra QUALQUER container, não só Postgres gerenciado: soma dos volumes nomeados montados quando tem, senão cai pro tamanho da camada gravável (`SizeRw`, via `docker system df`) — nunca usa `SizeRootFs` porque esse conta a imagem base inteira, inflando o número de qualquer container que compartilhe imagem com outro.

Auto-descoberta (via botão "Procurar servidores") ganhou um segundo caminho de entrada: clicar direto numa linha da tabela de containers do dashboard, quando ela parece Postgres e ainda não é gerenciada (badge "adotar"), abre o mesmo formulário de cadastro. Containers do próprio stack (`metadata-db`, `backend`, `frontend`, `docker-socket-proxy`) são sempre excluídos dessa detecção mesmo quando a imagem bate na heurística — `metadata-db` É um Postgres de verdade, só que é o interno da plataforma, nunca deveria virar "servidor gerenciado" adotável.

Todo gráfico (os 4 cards do dashboard e os 3 da aba Monitoramento de cada servidor: CPU/Memória/Conexões) agora é clicável — abre um modal com o gráfico ampliado e botões de período (5min/15min/30min/tudo). Importante: isso recorta o mesmo buffer em memória que já existe (~1h a 15s/amostra), não busca dado mais antigo no backend — não tem armazenamento de métricas de longo prazo no MVP, então "mudar o período" é só zoom dentro do que já foi coletado desde que o backend subiu.

---

## Gestão genérica de Docker (fora do escopo original — pedido explícito, 2026-07-18)

Pedido explícito do usuário, fora de ordem em relação ao MVP original ("não é só Postgres, quero gerenciar Docker de forma genérica") — autorização ampla, decidi arquitetura e implementei sem pausar. Plano completo em [docs/INFRA_PLAN.md](./docs/INFRA_PLAN.md). Nova seção "Docker" na tela inicial (`/infra`), separada da gestão de servidores Postgres.

- [x] ~~Containers/networks/volumes genéricos~~ (start/stop/restart/remover qualquer container do host, não só servidores gerenciados; criar/remover rede e volume; ver logs) — reaproveita o `internal/docker` (Engine API via docker-socket-proxy) que já era genérico por baixo, só a camada `server` que era amarrada a "servidor cadastrado". Proteções: nunca deixa apagar as redes/volumes fixos da própria plataforma nem o volume de dados de um servidor gerenciado (esse tem fluxo próprio, com sua confirmação).
- [x] ~~Deploy via `docker-compose.yml`~~ e ~~build via Dockerfile~~ (com ou sem contexto extra via upload `.tar`/`.tar.gz`) — backend chama `docker compose`/`docker build` via `os/exec` de dentro do próprio container (CLI instalado na imagem, fala pelo mesmo `DOCKER_HOST` do proxy) em vez de reimplementar orquestração de compose na unha. Nova categoria `BUILD` no docker-socket-proxy — `EXEC` continua desligado, nunca dá shell dentro de container gerenciado.
- [x] ~~Traefik (reverse proxy) + domínio + SSL automático via Let's Encrypt~~ — container gerenciado, rotas via file provider (arquivo YAML por domínio, nunca precisa recriar o container alvo pra rotear), ACME HTTP-01 (só precisa da porta 80 alcançável, sem credencial de DNS). Provider Docker do próprio Traefik foi cortado — exigiria dar a ele leitura no Docker que ele não tem por design, e o file provider já cobre tudo que essa tela precisa.
- [x] ~~Firewall do host (ufw)~~ — única peça que não dava pra fazer só com docker-socket-proxy, porque `ufw` mexe no namespace de rede do HOST, não do container. Solução: `firewall-agent/` é um binário Go separado (módulo próprio), roda direto no host via systemd (nunca em container), escuta só num socket Unix, só expõe listar/liberar/remover regra — nunca `ufw enable/disable/reset`, e a porta 22/tcp (SSH) nunca pode ser tocada por essa API em hipótese nenhuma (travado no código do agente). `setup.sh` instala ufw e libera 22/tcp **antes** de habilitar, ordem crítica pra nunca travar acesso SSH remoto.

Achado nessa leva: `ufw status` não mostra nada enquanto o firewall tá inativo, mesmo com regra já gravada (`ufw show added` confirma que existe) — o agente cai pro `show added` quando detecta `ufw` inativo, senão a listagem mentia "vazio" no primeiro uso, antes do primeiro `ufw enable`.

Testado ponta a ponta no droplet: stack real via compose (nginx respondendo na porta publicada), build com Dockerfile puro e com contexto via upload (arquivo copiado de verdade confirmado dentro da imagem), rota de domínio via Traefik roteando de verdade (`curl` com `Host:` forjado, sem DNS real) pro container alvo, e firewall-agent com `ufw` instalado mas deliberadamente **nunca habilitado** durante o teste (`ufw allow`/`delete` funcionam no ruleset independente do estado ativo/inativo) — testado assim de propósito pra nunca arriscar travar o próprio acesso SSH usado pra testar tudo isso.

### Leva 2 — terminal, file manager, login, create flow rico, Traefik mais rico (2026-07-20)

Pedido explícito do usuário, de novo fora de ordem, autorização ampla igual à leva 1. **Ainda não verificado ponta a ponta num droplet real** (essa sessão não tinha acesso a Docker/droplet, só `go build`/`go vet` cross-compilados pra linux + `next build` de produção — os dois limpos, mas isso cobre só compilação, não comportamento em runtime). Próximo passo antes de considerar isso "pronto" de verdade: `git pull` + `./setup.sh` no droplet e testar cada pedaço abaixo na UI.

- **Login/senha** (fecha o item pendente do MVP, `internal/auth` — sessão em `admin_sessions` no banco, não em memória, sobrevive a restart do backend). Motivo de ter entrado nessa leva mesmo sem pedido explícito: dar terminal e file manager do host pra API sem nenhuma autenticação (o estado de antes) seria fazer superfície de ataque séria sem nenhum controle de acesso — decisão tomada sem confirmação do usuário a tempo (pergunta feita, sem resposta em 60s), registrada explicitamente como julgamento próprio. `ADMIN_PASSWORD` gerada pelo `setup.sh` igual à `CREDENTIAL_ENCRYPTION_KEY`, ecoada uma vez no fim do setup. CORS teve que sair de `Access-Control-Allow-Origin: *` pra reflexão de Origin (cookie de sessão exige `Allow-Credentials: true`, incompatível com `*`).
- **Terminal web dentro de container** (aba de detalhe > Terminal, xterm.js + WebSocket) — exige categoria `EXEC` ligada no docker-socket-proxy, decisão que a leva 1 tinha tomado como "nunca liga, de propósito". Revertida aqui por pedido explícito e consciente do usuário, aceitando o trade-off (quem alcança a UI autenticada roda qualquer comando em qualquer container do host).
- **Criar container**: fluxo de "Novo container" virou 4 modos (Imagem/Dockerfile/Compose/Git) num dialog só, com editor de variável de ambiente e porta que faltava no modo Imagem. Modo Git clona repositório (público ou privado via credencial SSH/PAT cadastrada, `internal/infra/git_credentials.go`) e builda a partir do Dockerfile clonado — reaproveita o mesmo `runBuild` do build via upload, só troca como o contexto chega no disco.
- **Página de detalhe do container** (clicar no nome na lista) — inspect completo, gráfico de CPU/memória/rede (histórico por container é sob demanda: só começa a coletar quando alguém abre a aba de Estatísticas daquele container, e para sozinho depois de 10min sem leitura — diferente do histórico da plataforma, que é sempre ligado, porque "todo container do host" não tem limite), variáveis de ambiente, logs, redes (conectar/desconectar ao vivo) e volumes.
- **Gerenciador de arquivos, dois sabores**:
  - Dentro de container/volume: usa a API de archive do Docker (mesmo mecanismo do `pg_hba.conf`, sem exec) pra listar/ler/escrever/upload/baixar; **exclusão é a única operação que não tem equivalente na API de archive** e cai pro exec síncrono (`rm -rf`) — dependência de EXEC documentada de propósito, já que o resto do file manager evita EXEC por design. Volume solto (sem container) é gerenciado subindo um container `alpine` descartável com o volume montado, roda a operação, remove — mesmo mecanismo de container por baixo, sem um segundo caminho de código.
  - Do host: pasta fixa e configurável (`HOST_FILES_ROOT`, default `/srv/gestpg-files`, **nunca a raiz `/` do filesystem** — decisão própria, não pedida explicitamente, pra não virar root via browser) montada read-write no backend. Escrever/enviar/excluir exige reconfirmar a senha antes (`POST /api/v1/auth/step-up`, eleva a sessão por 5 minutos) — listar/ler/baixar não.
- **Traefik**: rotas ganharam caminho (path prefix + strip), modo redirecionamento (em vez de proxy, com 301/302 configurável) e redirecionamento automático http→https quando TLS tá ligado — que também conserta um bug que já existia: com TLS ligado e sem essa opção nova, a porta 80 não tinha nenhum router pra aquele domínio e dava 404 em vez de redirecionar. TCP/UDP passthrough (ex: expor Postgres cru por trás do Traefik) ficou de fora de propósito — precisaria recriar o container do Traefik (interrompe TODO o roteamento HTTP) só pra abrir um entrypoint novo, risco desproporcional ao pedido.
- Porta padrão do frontend mudou de 3000 pra **4173** (3000 colide com um monte de coisa comum) — `docker-compose.yml`, `Dockerfile`, `setup.sh`, `README.md` todos atualizados.

### Leva 3 — RBAC, recursos, cron, backup de volume, deploy automático via webhook (2026-07-20)

Pedido explícito (usuário comparou com EasyPanel e pediu pra fechar 5 gaps específicos), mesma sessão da Leva 2, mesmo "ainda não verificado num droplet real" — só `go build`/`go vet` (linux) + `next build` limpos.

- **Multi-usuário com 2 papéis** (`admin`/`viewer`) — fecha a lacuna de RBAC do backlog, só que na versão mínima: sem permissão por servidor/container individual (isso continua backlog). `admin_user` (singleton da Leva 2) virou tabela `users` de verdade (migration própria migra o admin existente, nunca edita a migration antiga já commitada). Regra de acesso é só UMA: qualquer método não-GET exige `admin` — cobre as ~150+ rotas da API sem precisar marcar rota por rota, com exceção do terminal (WebSocket via GET, mas dá controle total, então é admin-only mesmo sendo leitura tecnicamente). `viewer` consegue fazer logout mesmo sem ser admin (ação sobre a própria sessão, não sobre dado da plataforma).
- **Limite de CPU/memória pra container genérico** — mesmo campo que servidor Postgres gerenciado já tinha, agora também no fluxo Imagem/Dockerfile/Git do "Novo container", e editável ao vivo na aba Visão Geral do detalhe (reaproveita `docker.Client.UpdateContainerResources`, que já existia só pro Postgres).
- **Cron job genérico por container** — roda um comando shell (`sh -c`) num container num horário/intervalo, aba "Cron" no detalhe do container. "Cron básico" de novo (mesmo espírito do `backup_policies`): intervalo em minutos OU diário/semanal + horário UTC, sem parser de expressão cron de verdade. Depende de EXEC (já ligado desde a Leva 2).
- **Backup genérico de volume** — snapshot manual `.tar.gz` de qualquer volume nomeado (botão na aba Volumes), reaproveitando o mesmo container `alpine` descartável do file manager pra ler o volume via API de archive (sem exec) e comprimindo em streaming direto pro disco. Sem agendamento nessa leva (diferente do backup de Postgres) — só manual, guardado em `generic_backups_data` (volume próprio, nunca bind mount do host).
- **Deploy automático via webhook** — persiste a config de clone+build do modo Git (`git_deployments`, diferente do disparo único que já existia) e reimplanta o container sozinho a cada push. **Decisão de segurança relevante**: o endpoint de webhook (`POST .../git-deployments/{id}/webhook`) é a PRIMEIRA rota de toda a API que não passa por sessão de usuário — GitHub/GitLab não têm como mandar cookie de login, então autenticação ali é só a assinatura do provedor (HMAC-SHA256 `X-Hub-Signature-256` do GitHub, comparação direta de `X-Gitlab-Token` do GitLab) contra um segredo aleatório de 32 bytes gerado por deployment, cifrado no banco, devolvido em texto puro só uma vez na criação. Isso é padrão de mercado pra webhook (nenhum provedor consegue autenticar via sessão), mas ainda assim é uma mudança de superfície da API que o classificador de segurança do harness bloqueou a primeira tentativa de build até eu confirmar explicitamente — usuário não respondeu a tempo, segui com o julgamento próprio (opção recomendada: manter o webhook, mitigação já embutida) e sinalizei isso de volta pro usuário.

---

## Backlog pós-MVP ("perfumarias")

Não implementar agora. Detalhe completo em REQUISITOS.md. Resumo do que fica pra depois:

- Multi-storage de backup (Google Drive, S3, Dropbox, FTP), backup físico incremental com PITR real via pgBackRest/Barman (não só `pg_dump` lógico), criptografia de backup, teste automático de restore agendado (restaurar periodicamente pra validar que o backup presta)
- Todos os ~150 parâmetros do `postgresql.conf` (hoje só o subset essencial), editor de arquivo puro, perfis de workload (OLTP/OLAP), `pg_ident.conf`
- `pg_hba.conf` com drag-and-drop e simulador de regra
- Particionamento, RLS/policies, event triggers, FDW, replicação lógica (publications/subscriptions), tipos customizados/domains, tablespaces
- Índices: sugestão de faltantes/não usados (baseado em `pg_stat_statements`), rebuild concorrente
- EXPLAIN visual gráfico (plano de execução legível, não texto cru), autocomplete no editor SQL, queries salvas/compartilhadas
- Monitoramento avançado: locks/deadlock graph, replicação (réplicas/lag/slots), vacuum progress, detecção de bloat (tabelas/índices inchados por vacuum atrasado, com sugestão de ação), alerta de wraparound (`age(datfrozenxid)`), health score, correlação de log com métricas (ver log do Postgres no mesmo lugar que o gráfico de CPU/conexões daquele horário), previsão de capacidade (tendência de crescimento de disco)
- Tuning assistido de autovacuum e memória (sugerir `shared_buffers`/`work_mem`/etc. baseado no hardware real do container)
- Alertas configuráveis multi-canal (email/Slack/Discord/Telegram/webhook): conexões perto do limite, réplica atrasando, disco enchendo, queries travadas, deadlocks
- RBAC granular por recurso (ver/editar/só monitorar POR servidor/container individual — hoje só existe admin-vs-viewer global, ver "Leva 3"), 2FA, SSO, API keys, auditoria completa da plataforma (quem mudou qual config, quando, com opção de reverter — pgAudit no lado do banco + log próprio da plataforma), rotação de credenciais/secrets (senha de superuser não deveria ficar estática pra sempre)
- Certificados TLS geridos automaticamente (emissão e renovação) pras conexões Postgres
- HA (Patroni) com failover automático (promoção de réplica se a primária cair), connection pooling gerenciado (PgBouncer/PgCat), read replicas via UI com roteamento assimétrico (escrita → primária, leitura → réplicas, não round-robin)
- API REST pública documentada, CLI, Terraform provider, IaC export
- Multi-tenancy (organizações)
- Auto-descoberta de Postgres já existentes na máquina (não criados pelo sistema) — ao instalar, varrer a máquina local por instalações Postgres e containers de banco já rodando, sugerir cadastro, pedir senha de cada um se necessário. Sempre local — sem gestão remota por enquanto (possível "cloud" futuro que agregue múltiplas máquinas monitoradas fica pra depois)
- Upgrade de versão maior via wizard (`pg_upgrade`), clonagem rápida de banco (copy-on-write, ambiente de teste idêntico à produção em segundos — tipo Neon), mascaramento de dados (anonimizar dados sensíveis ao clonar produção pra dev/staging), retenção e arquivamento (política automática de quando arquivar/deletar dados antigos)
- Extensões avançadas com UI dedicada (habilitar/desabilitar genérico já cobre instalar; UI dedicada seria pra gerenciar o conteúdo — jobs do pg_cron, políticas do pgaudit, etc): `pgaudit`, `timescaledb`, `pg_partman`, `postgis` (`pgvector` e `pg_cron` já saíram — ver seção Extensões acima)

## Notas pro Claude

- Sempre que fechar um item do MVP, volta nesse arquivo e risca.
- Se usuário pedir algo do backlog antes do MVP fechar, implementa mas pergunta antes se é intencional priorizar fora de ordem.
- Não criar abstração/config pra funcionalidade do backlog "pra facilitar depois" — YAGNI, o MVP é pra sair rápido.
