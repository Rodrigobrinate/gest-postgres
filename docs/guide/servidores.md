# Servidores

Um **servidor gerenciado** é uma instância PostgreSQL provisionada (ou adotada) por esta plataforma. Cada um roda em container próprio, com volume nomeado e conectado à rede `gestpg-managed`.

## Criar servidor

**Servidores → Novo servidor**, informando:

- Nome
- Versão do Postgres (13 a 17)
- Usuário/senha do superuser
- Porta publicada
- Preset de recursos: **Pequeno / Médio / Grande** — calcula automaticamente `max_connections`, `shared_buffers`, `work_mem`, `maintenance_work_mem`, `effective_cache_size` (ver [Configuração](configuracao.md))

Cada servidor recebe volume nomeado próprio (dado sobrevive a recriação de container) e roda na imagem `gestpg-postgres:X` (Postgres oficial + `pgvector` + `pg_cron`, ver [Arquitetura](arquitetura.md#imagem-postgres-customizada)).

## Listar e status

A lista mostra status (rodando/parado/erro), versão e **conexões ativas** — esse número reaproveita o coletor de histórico que já amostra `pg_stat_activity` a cada ~15s pra aba Monitoramento; listar servidores não abre conexão nova nenhuma só pra mostrar o número.

## Start / Stop / Restart

Direto pela lista ou pela página de detalhe do servidor.

## Editar servidor

- **Nome e recursos (CPU/memória)** aplicam na hora — `ContainerUpdate` do Docker troca os limites de um container **rodando**, sem recriar, sem derrubar conexão.
- **Porta** é a exceção: Docker não permite trocar o binding de porta publicada sem recriar o container. Editar a porta para/remove (mantendo o volume)/recria com a porta nova — breve interrupção durante a troca.

## Excluir servidor

Pede confirmação e oferece manter ou apagar o volume de dados. Se o servidor tiver [connection pooling](pooling.md) habilitado, o container do PgBouncer é removido junto (best-effort — não trava a exclusão do Postgres se essa parte falhar).

## Connection string

Cada servidor mostra uma connection string pronta com a senha revelável sob clique — copiar direto pra conectar de fora (psql, DBeaver, etc).

## Auto-descoberta

Botão **Procurar servidores** na tela inicial lista containers do host que parecem Postgres e ainda não estão cadastrados na plataforma. Clicar em **Cadastrar** pede credenciais reais e só salva o registro depois de confirmar uma conexão de verdade (`SELECT` real — conecta na rede `gestpg-managed` primeiro se precisar). Nada fica registrado com senha errada.

Segundo caminho de entrada: no dashboard principal, clicar direto numa linha da tabela de containers que pareça Postgres e ainda não seja gerenciada (badge **adotar**) abre o mesmo formulário.

Containers do próprio stack (`metadata-db`, `backend`, `frontend`, `docker-socket-proxy`) são sempre excluídos da detecção, mesmo quando a imagem bate na heurística — `metadata-db` é um Postgres de verdade, mas é o interno da plataforma.

> Auto-descoberta **nunca** cria container ou volume novo — só passa a gerenciar o que já existe. Sem acesso ao host além da API Docker, Postgres nativo (fora de container) fica fora de alcance.

## SSL na conexão

Toda conexão que o backend faz a um servidor (validação de credencial na hora de adotar, e toda operação normal depois) usa `sslmode=prefer` — tenta TLS, cai pra texto puro se o alvo não oferecer. Isso permite adotar servidores de terceiros (ex.: atrás de um PgBouncer que exige TLS do cliente) sem quebrar nada contra os Postgres gerenciados por esta plataforma, que não têm SSL configurado.
