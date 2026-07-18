module github.com/gest-postgres/backend

go 1.25.0

// Versões pinadas de propósito: a API do docker/docker client reorganiza
// tipos entre api/types e api/types/{container,network,volume,image} em
// versões diferentes. v24.0.9 é a que o código em internal/docker foi
// escrito contra. Rodar `go mod tidy` resolve as dependências indiretas
// e gera o go.sum (não commitado ainda — sem toolchain Go nesta máquina
// pra gerar os checksums corretamente).
require (
	github.com/docker/docker v24.0.9+incompatible
	github.com/docker/go-connections v0.5.0
	github.com/jackc/pgx/v5 v5.5.5
)

// docker/docker/client importa github.com/pkg/errors (não a "errors" da stdlib)
// e usa errors.As/errors.Is nela — só existem a partir da v0.9.1. Sem essa pin,
// `go mod tidy` resolve pra v0.8.1 (2016) e o build quebra com "undefined: errors.As".
require github.com/pkg/errors v0.9.1 // indirect

require (
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sys v0.15.0
)

require (
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/distribution/reference v0.0.0-00010101000000-000000000000 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/morikuni/aec v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)

// docker/distribution@v2.8.3's compat shim (reference_deprecated.go) chama
// reference.SplitHostname, removida em distribution/reference v0.6.0. Sem isso
// `go mod tidy` puxa a v0.6.0 como "latest" e o build quebra com
// "undefined: reference.SplitHostname". v0.5.0 ainda tem a função.
replace github.com/distribution/reference => github.com/distribution/reference v0.5.0
