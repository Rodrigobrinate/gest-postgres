# Perguntas frequentes / troubleshooting

## Botão "Atualizar agora" não aparece

Só aparece quando há atualização disponível **e** o `update-agent` está instalado e rodando no host — droplets que ainda não passaram pelo `setup.sh` desta versão caem de volta pro comportamento antigo (só o comando pra copiar). Rode `sudo ./setup.sh` de novo (idempotente) pra instalar o agente, depois recarregue a checagem de atualização.

## "Atualizar agora" ficou travado em "atualizando" pra sempre

Provável reinício do host (ou queda do processo) no meio da atualização. `GET /api/v1/update/status` se autocura sozinho detectando que a unit systemd da execução não existe mais — reabra o diálogo de atualização, deve mostrar "interrompida" com botão pra tentar de novo. Se continuar travado, confira `systemctl status gestpg-update-agent` no host.

## Editei um parâmetro e não fez efeito

Confira o badge de "requer restart" no formulário de [Configuração](configuracao.md) — alguns parâmetros do Postgres não aceitam `pg_reload_conf()`, só pegam depois de um restart do servidor. Se já editou antes dessa correção existir no projeto (instalação muito antiga), veja [Arquitetura → fluxo de configuração](arquitetura.md#fluxo-de-configuração-do-postgres).

## Login não funciona logo após instalar

Confira se `.env` já existia de uma instalação anterior ao commit que passou `ADMIN_PASSWORD` pro container do backend — nesse caso o backend gera senha própria, só logada dentro do container:

```bash
docker compose logs backend | grep -i senha
```

Ou apague a senha do `.env` e rode `sudo ./setup.sh` de novo pra gerar uma nova (idempotente, não quebra o resto).

## Gráfico de container genérico nunca populava

Corrigido — coletor de histórico por container tinha um `context.Background()` sem timeout que podia travar o goroutine pra sempre, silenciosamente. Se ainda acontecer, confira se o backend está atualizado (`git log` — corrigido junto do bug de `blkio_stats` cgroup v2). Ver [Docker genérico](docker-infra.md#página-de-detalhe-do-container).

## `pg_cron` habilitado mas não funciona

`pg_cron` precisa de `shared_preload_libraries` + restart — clicar **Habilitar** na aba Extensões já cuida disso, mas leva um tempo (reinício do container). Confira o badge **"requer restart"** antes de clicar. Ver [Extensões](extensoes.md#pgvector-e-pg_cron).

## Servidor sobe em crash loop depois de habilitar 2 extensões que precisam de preload

Esse era um bug **do próprio Postgres** com `shared_preload_libraries` gravado como string única — já corrigido nesta plataforma (grava multi-valor, não string com vírgula). Ver [Extensões → bug do Postgres corrigido](extensoes.md#bug-do-postgres-corrigido-nesse-fluxo). Se acontecer, é sinal de instalação desatualizada.

## Não consigo restringir a porta 28080/4173 com `ufw`

Precisa da cadeia `DOCKER-USER` apontada pra `ufw-user-forward` — o `setup.sh` atual já faz isso, mas instalações antigas podem não ter. Rode `sudo ./setup.sh` de novo (idempotente) pra aplicar.

## Adotei um container Postgres de terceiro e deu erro de SSL

Antigo bug: conexão sempre usava `sslmode=disable`, falhando contra qualquer Postgres/PgBouncer que exigisse TLS do cliente. Corrigido pra `sslmode=prefer`. Se ainda acontecer, confira se está numa instalação atualizada.

## Onde fica o que ainda falta / backlog

Ver [`CLAUDE.md`](https://github.com/Rodrigobrinate/gest-postgres/blob/main/CLAUDE.md) no repositório — seção "Backlog pós-MVP" lista tudo que é intencionalmente fora de escopo por enquanto (multi-storage de backup, PITR real, réplicas/HA, RBAC granular por recurso, 2FA, etc.).
