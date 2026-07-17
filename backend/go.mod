module github.com/gest-postgres/backend

go 1.22

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

// docker/distribution@v2.8.3's compat shim (reference_deprecated.go) chama
// reference.SplitHostname, removida em distribution/reference v0.6.0. Sem isso
// `go mod tidy` puxa a v0.6.0 como "latest" e o build quebra com
// "undefined: reference.SplitHostname". v0.5.0 ainda tem a função.
replace github.com/distribution/reference => github.com/distribution/reference v0.5.0
