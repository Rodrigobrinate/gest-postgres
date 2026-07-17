package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

func NewRouter(serverService *server.Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	servers := NewServersHandler(serverService)
	mux.HandleFunc("POST /api/v1/servers", servers.Create)
	mux.HandleFunc("GET /api/v1/servers", servers.List)
	mux.HandleFunc("GET /api/v1/servers/{id}", servers.Get)
	mux.HandleFunc("POST /api/v1/servers/{id}/start", servers.Start)
	mux.HandleFunc("POST /api/v1/servers/{id}/stop", servers.Stop)
	mux.HandleFunc("POST /api/v1/servers/{id}/restart", servers.Restart)
	mux.HandleFunc("DELETE /api/v1/servers/{id}", servers.Delete)

	return withCORS(withLogging(mux))
}
