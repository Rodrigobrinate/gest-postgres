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

	detail := NewDetailHandler(serverService)
	mux.HandleFunc("GET /api/v1/servers/{id}/databases", detail.Databases)
	mux.HandleFunc("GET /api/v1/servers/{id}/tables", detail.Tables)
	mux.HandleFunc("GET /api/v1/servers/{id}/tables/{schema}/{table}/rows", detail.TableRows)
	mux.HandleFunc("POST /api/v1/servers/{id}/query", detail.Query)
	mux.HandleFunc("GET /api/v1/servers/{id}/activity", detail.Activity)
	mux.HandleFunc("POST /api/v1/servers/{id}/activity/{pid}/cancel", detail.CancelBackend)
	mux.HandleFunc("POST /api/v1/servers/{id}/activity/{pid}/terminate", detail.TerminateBackend)
	mux.HandleFunc("GET /api/v1/servers/{id}/logs", detail.Logs)
	mux.HandleFunc("GET /api/v1/servers/{id}/stats", detail.Stats)

	return withCORS(withLogging(mux))
}
