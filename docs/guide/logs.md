# Logs

Aba **Logs** de cada servidor — visualizador do log do Postgres, com parsing estruturado (visual inspirado no Observability da Cloudflare).

## O que faz

- **Nível extraído do texto por regex** (LOG/ERROR/WARNING/FATAL/PANIC/NOTICE/INFO/DEBUG) — não da posição na linha, já que `log_line_prefix` é editável e não tem posição fixa garantida.
- **Cor por nível**, badge.
- **Linhas de continuação** do Postgres (`DETAIL`/`HINT`/`STATEMENT`/`CONTEXT`/`QUERY`) agrupadas sob a linha principal, só visíveis ao expandir.
- **Filtro por nível** (chips) e **por texto livre**.

Sem armazenamento de longo prazo — é tail sobre o log corrente do container, não um índice pesquisável histórico.
