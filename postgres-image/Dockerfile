# Imagem Postgres gerenciada pela plataforma — estende a imagem oficial (Debian,
# não a variante alpine) com extensões que o Postgres puro não traz: pgvector
# (embeddings/busca vetorial) e pg_cron (job scheduler dentro do banco).
#
# A imagem oficial já vem com o repositório APT da PGDG configurado (é de lá
# que ela mesma instala o postgresql-$PG_MAJOR), então os pacotes abaixo
# instalam sem precisar adicionar fonte nova.
#
# Build local (fora do docker-socket-proxy — feito pelo setup.sh como root,
# direto no daemon do host): o backend nunca precisa de permissão de build,
# só de pull, e essa imagem já existe localmente quando ele for criá-la.
ARG PG_MAJOR=17
FROM postgres:${PG_MAJOR}
ARG PG_MAJOR=17

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        postgresql-${PG_MAJOR}-pgvector \
        postgresql-${PG_MAJOR}-cron \
    && rm -rf /var/lib/apt/lists/*
