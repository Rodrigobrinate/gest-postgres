// Package version guarda o commit em que o binário foi buildado — setado via
// ldflags no build (ver Dockerfile/docker-compose.yml/setup.sh), nunca lido
// em runtime, pra UI conseguir mostrar "atualização disponível" comparando
// com o HEAD do GitHub sem precisar de um arquivo de versão mantido à mão.
package version

// Commit é sobrescrito em build time via:
//
//	-ldflags "-X github.com/gest-postgres/backend/internal/version.Commit=<sha>"
//
// "dev" quando buildado sem GIT_COMMIT (ex: `go run`, `docker compose up
// --build` manual sem passar pelo setup.sh).
var Commit = "dev"
