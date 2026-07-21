# Connection pooling (PgBouncer)

Card na aba **Configuração** de cada servidor — toggle **Habilitar**/**Desabilitar**.

## Como funciona

Sobe um container `edoburu/pgbouncer` companheiro por servidor (nome `{container}-pgbouncer`), **sem volume** — é stateless, a imagem gera `pgbouncer.ini`/`userlist.txt` sozinha a partir de env vars no boot. Publica numa porta própria, alocada da mesma faixa dos servidores.

Cliente troca só a **porta** (mesmo usuário/senha/banco) e passa a falar com o pooler em vez do Postgres direto.

## Modo de pool

Escolhível: `transaction` (default), `session`, `statement`.

## Autenticação

`AUTH_TYPE=scram-sha-256` de propósito. `AUTH_TYPE=md5` (default mais comum em tutoriais) quebra contra qualquer role criada com o `password_encryption` default do Postgres desde a v10+ (`scram-sha-256`) — erro "wrong password type" tanto cliente→PgBouncer quanto PgBouncer→Postgres.

## Ciclo de vida

- **Excluir o servidor** remove o pooler junto (best-effort — não trava a exclusão do Postgres se falhar).
- **Rotacionar a senha do superuser** recria o container do pooler com a senha nova — sem isso o pooler ficaria autenticando com credencial velha silenciosamente (mesma classe de bug já corrigida na rotação de senha em si).
- **Trocar a porta do Postgres** (editar servidor) **não** precisa recriar o pooler — ele fala com o Postgres pelo nome do container, não pela porta publicada.
