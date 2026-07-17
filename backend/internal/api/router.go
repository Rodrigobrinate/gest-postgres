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
	mux.HandleFunc("GET /api/v1/servers/{id}/password", detail.Password)
	mux.HandleFunc("GET /api/v1/servers/{id}/databases", detail.Databases)
	mux.HandleFunc("GET /api/v1/servers/{id}/database-sizes", detail.DatabaseSizes)
	mux.HandleFunc("GET /api/v1/servers/{id}/metrics-history", detail.MetricsHistory)
	mux.HandleFunc("GET /api/v1/servers/{id}/tables", detail.Tables)
	mux.HandleFunc("POST /api/v1/servers/{id}/tables", detail.CreateTable)
	mux.HandleFunc("GET /api/v1/servers/{id}/tables/{schema}/{table}/rows", detail.TableRows)
	mux.HandleFunc("POST /api/v1/servers/{id}/query", detail.Query)
	mux.HandleFunc("GET /api/v1/servers/{id}/activity", detail.Activity)
	mux.HandleFunc("POST /api/v1/servers/{id}/activity/{pid}/cancel", detail.CancelBackend)
	mux.HandleFunc("POST /api/v1/servers/{id}/activity/{pid}/terminate", detail.TerminateBackend)
	mux.HandleFunc("GET /api/v1/servers/{id}/logs", detail.Logs)
	mux.HandleFunc("GET /api/v1/servers/{id}/stats", detail.Stats)
	mux.HandleFunc("GET /api/v1/servers/{id}/roles", detail.ListRoles)
	mux.HandleFunc("POST /api/v1/servers/{id}/roles", detail.CreateRole)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/roles/{name}", detail.DropRole)
	mux.HandleFunc("GET /api/v1/servers/{id}/roles/{name}/privileges", detail.RolePrivileges)
	mux.HandleFunc("POST /api/v1/servers/{id}/roles/{name}/privileges/{schema}/{table}/{privilege}/grant", detail.GrantPrivilege)
	mux.HandleFunc("POST /api/v1/servers/{id}/roles/{name}/privileges/{schema}/{table}/{privilege}/revoke", detail.RevokePrivilege)
	mux.HandleFunc("GET /api/v1/servers/{id}/triggers", detail.ListTriggers)
	mux.HandleFunc("GET /api/v1/servers/{id}/trigger-functions", detail.ListTriggerFunctions)
	mux.HandleFunc("POST /api/v1/servers/{id}/triggers", detail.CreateTrigger)
	mux.HandleFunc("POST /api/v1/servers/{id}/triggers/{schema}/{table}/{name}/enable", detail.EnableTrigger)
	mux.HandleFunc("POST /api/v1/servers/{id}/triggers/{schema}/{table}/{name}/disable", detail.DisableTrigger)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/triggers/{schema}/{table}/{name}", detail.DropTrigger)
	mux.HandleFunc("GET /api/v1/servers/{id}/slow-queries", detail.SlowQueries)
	mux.HandleFunc("POST /api/v1/servers/{id}/slow-queries/reset", detail.ResetQueryStats)
	mux.HandleFunc("POST /api/v1/servers/{id}/query-stats/enable", detail.EnableQueryStats)
	mux.HandleFunc("GET /api/v1/servers/{id}/config", detail.GetExpandedConfig)
	mux.HandleFunc("PUT /api/v1/servers/{id}/config", detail.PutExpandedConfig)
	mux.HandleFunc("GET /api/v1/servers/{id}/extensions", detail.Extensions)
	mux.HandleFunc("POST /api/v1/servers/{id}/extensions/{name}/enable", detail.EnableExtension)
	mux.HandleFunc("POST /api/v1/servers/{id}/extensions/{name}/disable", detail.DisableExtension)

	mux.HandleFunc("GET /api/v1/servers/{id}/views", detail.ListViews)
	mux.HandleFunc("POST /api/v1/servers/{id}/views", detail.CreateView)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/views/{schema}/{name}", detail.DropView)

	mux.HandleFunc("GET /api/v1/servers/{id}/materialized-views", detail.ListMaterializedViews)
	mux.HandleFunc("POST /api/v1/servers/{id}/materialized-views", detail.CreateMaterializedView)
	mux.HandleFunc("POST /api/v1/servers/{id}/materialized-views/{schema}/{name}/refresh", detail.RefreshMaterializedView)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/materialized-views/{schema}/{name}", detail.DropMaterializedView)

	mux.HandleFunc("GET /api/v1/servers/{id}/sequences", detail.ListSequences)
	mux.HandleFunc("POST /api/v1/servers/{id}/sequences", detail.CreateSequence)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/sequences/{schema}/{name}", detail.DropSequence)

	mux.HandleFunc("GET /api/v1/servers/{id}/types", detail.ListTypes)
	mux.HandleFunc("POST /api/v1/servers/{id}/types/enum", detail.CreateEnumType)
	mux.HandleFunc("POST /api/v1/servers/{id}/types/domain", detail.CreateDomain)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/types/{schema}/{name}", detail.DropType)

	mux.HandleFunc("GET /api/v1/servers/{id}/functions", detail.ListFunctions)
	mux.HandleFunc("POST /api/v1/servers/{id}/functions", detail.CreateFunction)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/functions/{schema}/{name}", detail.DropFunction)

	return withCORS(withLogging(mux))
}
