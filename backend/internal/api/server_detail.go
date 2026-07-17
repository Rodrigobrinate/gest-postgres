package api

import (
	"net/http"
	"strconv"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

// DetailHandler agrupa os endpoints que operam DENTRO de um servidor gerenciado
// (query, monitoramento, logs) — diferente de ServersHandler, que é só o
// ciclo de vida do container.
type DetailHandler struct {
	service *server.Service
}

func NewDetailHandler(service *server.Service) *DetailHandler {
	return &DetailHandler{service: service}
}

func (h *DetailHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	cfg, err := h.service.GetLiveConfig(r.Context(), id, database)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cfg)
}

func (h *DetailHandler) PutConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	var cfg server.PostgresConfig
	if err := httpx.DecodeJSON(r, &cfg); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}

	restartRequired, err := h.service.ApplyConfig(r.Context(), id, database, cfg)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"restart_required": restartRequired})
}

func (h *DetailHandler) Extensions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	extensions, err := h.service.ListExtensions(r.Context(), id, database)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, extensions)
}

func (h *DetailHandler) EnableExtension(w http.ResponseWriter, r *http.Request) {
	h.toggleExtension(w, r, true)
}

func (h *DetailHandler) DisableExtension(w http.ResponseWriter, r *http.Request) {
	h.toggleExtension(w, r, false)
}

func (h *DetailHandler) toggleExtension(w http.ResponseWriter, r *http.Request, enable bool) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	database := r.URL.Query().Get("database")

	var err error
	if enable {
		err = h.service.EnableExtension(r.Context(), id, database, name)
	} else {
		err = h.service.DisableExtension(r.Context(), id, database, name)
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) Password(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	password, err := h.service.Password(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"password": password})
}

func (h *DetailHandler) Databases(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	names, err := h.service.ListDatabases(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, names)
}

func (h *DetailHandler) Tables(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	tables, err := h.service.ListTables(r.Context(), id, database)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tables == nil {
		tables = []server.TableInfo{}
	}
	httpx.WriteJSON(w, http.StatusOK, tables)
}

func (h *DetailHandler) CreateTable(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	var in server.CreateTableInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}

	if err := h.service.CreateTable(r.Context(), id, database, in); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) TableRows(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")
	schema := r.PathValue("schema")
	table := r.PathValue("table")

	limit := parseIntDefault(r.URL.Query().Get("limit"), 50, 1, 500)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0, 0, 1_000_000_000)

	result, total, err := h.service.TableRows(r.Context(), id, database, schema, table, limit, offset)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"columns":     result.Columns,
		"rows":        result.Rows,
		"total_rows":  total,
		"limit":       limit,
		"offset":      offset,
		"duration_ms": result.DurationMs,
	})
}

type runQueryInput struct {
	Database string `json:"database"`
	SQL      string `json:"sql"`
}

func (h *DetailHandler) Query(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var in runQueryInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if in.SQL == "" {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "campo sql é obrigatório")
		return
	}

	result, err := h.service.RunQuery(r.Context(), id, in.Database, in.SQL)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *DetailHandler) Activity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	sessions, err := h.service.Activity(r.Context(), id, database)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if sessions == nil {
		sessions = []server.ActivitySession{}
	}
	httpx.WriteJSON(w, http.StatusOK, sessions)
}

func (h *DetailHandler) CancelBackend(w http.ResponseWriter, r *http.Request) {
	h.signalBackend(w, r, false)
}

func (h *DetailHandler) TerminateBackend(w http.ResponseWriter, r *http.Request) {
	h.signalBackend(w, r, true)
}

func (h *DetailHandler) signalBackend(w http.ResponseWriter, r *http.Request, terminate bool) {
	id := r.PathValue("id")
	pid, err := strconv.ParseInt(r.PathValue("pid"), 10, 32)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "pid inválido")
		return
	}

	if terminate {
		err = h.service.TerminateBackend(r.Context(), id, int32(pid))
	} else {
		err = h.service.CancelBackend(r.Context(), id, int32(pid))
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tail := parseIntDefault(r.URL.Query().Get("tail"), 500, 1, 5000)

	logs, err := h.service.Logs(r.Context(), id, tail)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

func (h *DetailHandler) Stats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stats, err := h.service.Stats(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

func parseIntDefault(raw string, fallback, min, max int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
