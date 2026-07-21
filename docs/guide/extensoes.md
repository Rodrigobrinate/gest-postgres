# Extensões

Aba **Extensões** de cada servidor.

## Listar e habilitar

Lista tudo de `pg_available_extensions`. Habilita/desabilita qualquer uma com um clique — não só as mais comuns (`pg_stat_statements`, `uuid-ossp`, `pgcrypto`, `pg_trgm`).

## `pgvector` e `pg_cron`

`postgres:X` oficial não vem com essas duas compiladas. Servidores novos sobem em cima de `gestpg-postgres:X` (ver [Arquitetura](arquitetura.md#imagem-postgres-customizada)) — a mesma imagem oficial + `pgvector`/`pg_cron` via apt.

- **`pgvector`** funciona na hora — `CREATE EXTENSION vector`, sem restart.
- **`pg_cron`** precisa de `shared_preload_libraries` + `cron.database_name` (mesmo tratamento que `pg_stat_statements` já tinha). Clicar em **Habilitar** na aba Extensões cuida disso sozinho — reinicia o container automaticamente, demora um pouco mais, badge **"requer restart"** avisa antes.

> Servidores criados antes dessa mudança continuam na imagem `postgres:X` antiga — sem `pgvector`/`pg_cron` disponíveis até serem recriados. Sem migração automática no MVP.

## Bug do Postgres corrigido nesse fluxo

`ALTER SYSTEM SET shared_preload_libraries = 'lib1,lib2'` (string única com vírgula dentro) faz o Postgres persistir **errado** em `postgresql.auto.conf` — grava `= '"lib1,lib2"'` com aspas duplas extras envolvendo tudo, e na subida seguinte ele tenta abrir um arquivo de lib chamado literalmente `lib1,lib2` e entra em crash loop permanente.

A sintaxe correta é multi-valor: `ALTER SYSTEM SET shared_preload_libraries = lib1, lib2` — identificadores soltos, sem string por fora. A plataforma sempre grava assim. Servidores com só 1 lib preload nunca bateram nesse bug (só aparece com 2+).

## Fluxo guiado de `pg_stat_statements`

Se a aba **Desempenho** detectar que a extensão está instalada mas não está coletando (não preloaded), oferece um fluxo guiado que reinicia o servidor com o preload correto — em vez de só mostrar "sem dados".
