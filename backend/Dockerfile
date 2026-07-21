FROM golang:1.25-alpine AS build
WORKDIR /src
# Imagem oficial trava em GOTOOLCHAIN=local; deps transitivas (ex: golang.org/x/time)
# podem exigir uma versão de Go mais nova que a da imagem base — deixa baixar sozinho.
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# GIT_COMMIT vem de fora (docker-compose.yml build.args, setup.sh exporta a
# partir do HEAD real) — o contexto de build aqui é só backend/, sem .git,
# então não dá pra descobrir o commit de dentro do Dockerfile.
ARG GIT_COMMIT=dev
RUN CGO_ENABLED=0 go build -ldflags "-X github.com/gest-postgres/backend/internal/version.Commit=${GIT_COMMIT}" -o /out/api ./cmd/api

FROM alpine:3.21
# postgresql17-client: pg_dump/pg_restore pro backup, roda direto do backend
# (conecta via rede na porta 5432 do container gerenciado, igual toda outra
# operação — nunca via docker exec, mesma decisão de superfície de ataque do
# resto do projeto). Cliente na versão mais nova suportada (17) porque
# pg_dump só consegue falar com servidor da MESMA versão ou mais velha, nunca
# mais nova — cobre os servidores v13-17 que a plataforma cria. Alpine 3.19
# só tinha até o pacote postgresql16-client, por isso o bump de versão da
# imagem base.
RUN apk add --no-cache ca-certificates postgresql17-client
# docker-cli + docker-cli-compose: pra deploy via docker-compose.yml e build
# via Dockerfile, o backend chama `docker compose`/`docker build` (via
# os/exec) em vez de reimplementar o parser de compose ou o protocolo de
# build na unha. DOCKER_HOST já aponta pro docker-socket-proxy (mesma env
# var que o client Go usa), então o CLI fala com o Docker pelo mesmo caminho
# mediado — nunca ganha acesso direto ao socket.
RUN apk add --no-cache docker-cli docker-cli-compose
# git + openssh-client: pra criar container a partir de um repositório Git
# (aba Docker > Novo container > Git) — clona via os/exec, mesmo raciocínio
# do docker-cli acima. openssh-client é o `ssh` que GIT_SSH_COMMAND usa pra
# clone via chave privada de repositório privado.
RUN apk add --no-cache git openssh-client
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
