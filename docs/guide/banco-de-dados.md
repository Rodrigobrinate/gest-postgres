# Banco de dados e objetos

## Databases

Aba **Monitoramento**, card "Bancos de dados": criar, listar, excluir.

- Excluir usa `DROP DATABASE ... WITH (FORCE)` (Postgres 13+) — derruba conexões abertas em vez de falhar.
- O banco principal do servidor não pode ser excluído.
- Nome do banco é validado (mesma regra usada no backup manual e agendado) — fecha path traversal e injeção de conninfo em comandos que recebem o nome como argumento.

## Tabelas

Aba **Tabelas**.

- Criar via formulário visual: nome, colunas, tipos, PK, not null, default.
- Listar, com botão de excluir que aparece ao passar o mouse na linha.
- **Ver dados em grid com paginação.**

## ERD

Aba **ERD** — diagrama entidade-relacionamento de todo schema não-sistema do banco selecionado: uma caixa por tabela (colunas com tipo, `PK`/`FK` marcados), linha conectando cada foreign key de verdade até a tabela referenciada. Introspecciona `information_schema` (colunas + chave primária + chave estrangeira, sem tocar dado de nenhuma tabela). Renderizado com [Mermaid](https://mermaid.js.org/) no navegador; zoom simples (botões +/−/reset) e atualizar sob demanda.

Assume relação 1-pra-N em toda FK (o caso disparado mais comum) — não checa se a coluna FK também tem `UNIQUE` (o que a tornaria 1-pra-1). Sem seleção manual de tabela no MVP: sempre o schema inteiro.

### Bug notável já corrigido

Coluna `id` com PK `uuid` chegou a mostrar array cru (`[176,70,128,...]`) em vez do UUID formatado, em qualquer tabela com PK `uuid`. Causa: `pgx` decodifica `uuid` pra `[16]byte` (array de tamanho fixo); a função de serialização (`jsonSafeValue`, compartilhada pelo Editor SQL e por "Ver dados da tabela") só tratava `[]byte` (slice) e `fmt.Stringer` — `[16]byte` caía no `default` e virava array de números crus no JSON. Corrigido com um case dedicado formatando `8-4-4-4-12`.

## Editor SQL

Aba **Editor SQL** — roda query livre, resultado em grid. Ganhou além do MVP original:

- Syntax highlighting (CodeMirror).
- Histórico de queries — guardado em `sessionStorage` (não `localStorage`), limpo explicitamente no logout, pra não sobreviver a troca de sessão.

## Objetos

Aba **Objetos** — além do MVP original:

- **Views** — criar/listar/excluir.
- **Materialized Views** — criar/listar/refresh/excluir.
- **Sequences** — criar/listar/excluir.
- **Types/Domains** — enum e domain com `CHECK`, criar/listar/excluir.

## Funções

Aba **Funções** — functions e procedures:

- Listar, com definição expansível via `pg_get_functiondef`.
- Criar via SQL cru num editor CodeMirror.
- Excluir — suporta overload (assinatura completa, não só o nome).

## Triggers

Gerenciados por tabela: criar/habilitar/desabilitar/excluir.

## Usuários e permissões

Aba **Usuários**:

- Criar/excluir role.
- Flags: login / superuser / createdb / createrole.
- Matriz de permissões: `GRANT`/`REVOKE` de `SELECT`/`INSERT`/`UPDATE`/`DELETE` por tabela.

## Desempenho

Aba **Desempenho** — queries lentas via `pg_stat_statements`:

- Ordenação por tempo total / tempo médio / número de chamadas.
- Reset de stats.
- Fluxo guiado de habilitação quando a extensão ainda não está coletando (ver [Extensões](extensoes.md)).
- Servidores novos já saem com `pg_stat_statements` pré-carregado em `shared_preload_libraries` — sem isso a extensão fica instalada mas não coleta nada.
