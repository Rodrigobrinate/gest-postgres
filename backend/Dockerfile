FROM golang:1.25-alpine AS build
WORKDIR /src
# Imagem oficial trava em GOTOOLCHAIN=local; deps transitivas (ex: golang.org/x/time)
# podem exigir uma versão de Go mais nova que a da imagem base — deixa baixar sozinho.
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

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
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
