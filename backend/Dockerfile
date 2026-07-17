FROM golang:1.25-alpine AS build
WORKDIR /src
# Imagem oficial trava em GOTOOLCHAIN=local; deps transitivas (ex: golang.org/x/time)
# podem exigir uma versão de Go mais nova que a da imagem base — deixa baixar sozinho.
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
