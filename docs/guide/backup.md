# Backup e restore

Aba **Backup** de cada servidor.

## Como o dump é feito

`pg_dump`/`pg_restore` rodam **direto de dentro do container do backend** — nunca via `docker exec` no container gerenciado (mesma decisão de segurança do resto do projeto: `docker-socket-proxy` nunca libera `EXEC` pra esse fim). O cliente instalado é a versão 17, que cobre dump de servidores v13 a v17 (`pg_dump` só fala com Postgres da mesma versão ou mais velho, nunca mais novo). Conecta pelo nome do container na rede `gestpg-managed`, igual toda outra operação do backend.

## Backup manual

`pg_dump`, formato **custom**. Botão na aba Backup.

## Rotina agendada

"Cron básico" é literal — **sem parser de expressão cron de verdade**. Só frequência (diária/semanal + dia da semana) e horário (UTC), checado a cada 1 minuto contra o `last_run_at` de cada política habilitada.

## Storage

- **Local** — arquivo num volume Docker próprio (`backups_data`), nunca bind mount do host.
- **Google Drive** — sem cópia local nenhuma. O dump escreve num arquivo temporário (`/backups/tmp`), sobe em streaming (multipart via `io.Pipe`, nunca carrega o arquivo inteiro em memória — dumps grandes podem ser vários GB) pra uma pasta própria (`gest-postgres-backups`) na conta configurada, e o temporário é apagado depois.

### Configurar Google Drive

Cada instalação usa o **próprio app OAuth do dono** (client_id/secret cadastrado nas configurações da plataforma) — nenhuma credencial Google embutida.

1. Backend gera URL de consentimento (`access_type=offline&prompt=consent`, garante `refresh_token` de verdade, não só `access_token` que expira em 1h).
2. Usuário autoriza no próprio navegador (não tem como automatizar — é a Google pedindo login da conta).
3. Callback troca o `code` pelo token; `refresh_token` é guardado **cifrado**.
4. `state` do fluxo OAuth é aleatório (32 bytes) com expiração de 10 minutos — fecha um CSRF de vinculação de conta (sem isso, um atacante conseguia induzir um admin logado a vincular a conta Google *dele*, redirecionando todo backup futuro pro Drive do atacante).

Implementado com `golang.org/x/oauth2` + chamadas HTTP diretas pra Drive API v3 REST, sem o SDK `google.golang.org/api` — evita puxar uma árvore de dependência bem maior só pra 3 operações (upload/download/delete).

## Restore

Dois modos:

- **Sobrescrever** um banco já existente — apaga tudo que tinha antes.
- **Criar um banco novo** do zero e restaurar nele, sem tocar no original.

Baixa (do Drive) ou abre direto (local) antes de rodar `pg_restore --clean --if-exists`.

## Retenção

`retention_count` — mantém últimos N backups. Só conta backups gerados **por aquela política especificamente**; um backup manual nunca é apagado automaticamente por nenhuma política agendada.

## Validação do nome do banco

O campo `database` (backup manual e política agendada) passa pela mesma validação (`identRegex`) usada em `CreateDatabase`/restore — fecha path traversal (o valor virava segmento de nome de arquivo) e injeção de conninfo no `pg_dump -d` (um valor tipo `dbname=x host=atacante.com` redirecionava a conexão, com `PGPASSWORD` no ambiente, pro host do atacante).
