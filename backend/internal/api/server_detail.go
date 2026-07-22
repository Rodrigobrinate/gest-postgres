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

type createDatabaseInput struct {
	Name string `json:"name"`
}

func (h *DetailHandler) CreateDatabase(w http.ResponseWriter, r *http.Request) {
	var in createDatabaseInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateDatabase(r.Context(), r.PathValue("id"), in.Name); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropDatabase(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DropDatabase(r.Context(), r.PathValue("id"), r.PathValue("name")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createTestDatabaseInput struct {
	Suffix string `json:"suffix"`
}

func (h *DetailHandler) CreateTestDatabase(w http.ResponseWriter, r *http.Request) {
	var in createTestDatabaseInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	result, err := h.service.CreateTestDatabase(r.Context(), r.PathValue("id"), in.Suffix)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, result)
}

func (h *DetailHandler) ListHbaRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListHbaRules(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rules)
}

func (h *DetailHandler) AddHbaRule(w http.ResponseWriter, r *http.Request) {
	var in server.AddHbaRuleInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.AddHbaRule(r.Context(), r.PathValue("id"), in); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

type deleteHbaRuleInput struct {
	Raw string `json:"raw"`
}

func (h *DetailHandler) DeleteHbaRule(w http.ResponseWriter, r *http.Request) {
	var in deleteHbaRuleInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.DeleteHbaRule(r.Context(), r.PathValue("id"), in.Raw); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DetailHandler) RotateSuperuserPassword(w http.ResponseWriter, r *http.Request) {
	password, err := h.service.RotateSuperuserPassword(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"password": password})
}

func (h *DetailHandler) RotateRolePassword(w http.ResponseWriter, r *http.Request) {
	password, err := h.service.RotateRolePassword(r.Context(), r.PathValue("id"), r.PathValue("name"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"password": password})
}

func (h *DetailHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	roles, err := h.service.ListRoles(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, roles)
}

func (h *DetailHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var in server.CreateRoleInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}

	if err := h.service.CreateRole(r.Context(), id, in); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")

	if err := h.service.DropRole(r.Context(), id, name); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DetailHandler) RolePrivileges(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	database := r.URL.Query().Get("database")

	privs, err := h.service.ListRolePrivileges(r.Context(), id, database, name)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, privs)
}

func (h *DetailHandler) GrantPrivilege(w http.ResponseWriter, r *http.Request) {
	h.setPrivilege(w, r, true)
}

func (h *DetailHandler) RevokePrivilege(w http.ResponseWriter, r *http.Request) {
	h.setPrivilege(w, r, false)
}

func (h *DetailHandler) setPrivilege(w http.ResponseWriter, r *http.Request, grant bool) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	schema := r.PathValue("schema")
	table := r.PathValue("table")
	privilege := r.PathValue("privilege")
	database := r.URL.Query().Get("database")

	if err := h.service.SetTablePrivilege(r.Context(), id, database, schema, table, name, privilege, grant); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type setAccessLevelInput struct {
	Level string `json:"level"`
}

// SetAccessLevel é o preset de alto nível (leitura/escrita/nenhum) da aba
// Usuários — aplica em todas as tabelas do banco de uma vez, ver
// SetDatabaseAccessLevel.
func (h *DetailHandler) SetAccessLevel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	database := r.URL.Query().Get("database")

	var in setAccessLevelInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.SetDatabaseAccessLevel(r.Context(), id, database, name, in.Level); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) ListTriggers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")

	triggers, err := h.service.ListTriggers(r.Context(), id, database, schema, table)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, triggers)
}

func (h *DetailHandler) ListTriggerFunctions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	names, err := h.service.ListTriggerFunctions(r.Context(), id, database)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if names == nil {
		names = []string{}
	}
	httpx.WriteJSON(w, http.StatusOK, names)
}

func (h *DetailHandler) CreateTrigger(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	var in server.CreateTriggerInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}

	if err := h.service.CreateTrigger(r.Context(), id, database, in); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) EnableTrigger(w http.ResponseWriter, r *http.Request) {
	h.setTriggerEnabled(w, r, true)
}

func (h *DetailHandler) DisableTrigger(w http.ResponseWriter, r *http.Request) {
	h.setTriggerEnabled(w, r, false)
}

func (h *DetailHandler) setTriggerEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")
	schema := r.PathValue("schema")
	table := r.PathValue("table")
	name := r.PathValue("name")

	if err := h.service.SetTriggerEnabled(r.Context(), id, database, schema, table, name, enabled); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropTrigger(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")
	schema := r.PathValue("schema")
	table := r.PathValue("table")
	name := r.PathValue("name")

	if err := h.service.DropTrigger(r.Context(), id, database, schema, table, name); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DetailHandler) SlowQueries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")
	orderBy := r.URL.Query().Get("order_by")

	queries, available, err := h.service.ListSlowQueries(r.Context(), id, database, orderBy)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if queries == nil {
		queries = []server.SlowQuery{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"available": available,
		"queries":   queries,
	})
}

func (h *DetailHandler) ResetQueryStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	if err := h.service.ResetQueryStats(r.Context(), id, database); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) EnableQueryStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	if err := h.service.EnableQueryStats(r.Context(), id, database); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) GetExpandedConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	params, err := h.service.GetExpandedConfig(r.Context(), id, database)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, params)
}

type applyConfigInput struct {
	Updates map[string]string `json:"updates"`
}

func (h *DetailHandler) PutExpandedConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	var in applyConfigInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}

	restartRequired, err := h.service.ApplyExpandedConfig(r.Context(), id, database, in.Updates)
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

type enablePoolingInput struct {
	PoolMode string `json:"pool_mode"`
}

func (h *DetailHandler) EnablePooling(w http.ResponseWriter, r *http.Request) {
	var in enablePoolingInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	updated, err := h.service.EnablePooling(r.Context(), r.PathValue("id"), in.PoolMode)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (h *DetailHandler) DisablePooling(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DisablePooling(r.Context(), r.PathValue("id")); err != nil {
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

func (h *DetailHandler) DatabaseSizes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sizes, err := h.service.DatabaseSizes(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sizes)
}

func (h *DetailHandler) MetricsHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rangeDur := server.ParseHistoryRange(r.URL.Query().Get("range"))
	points, err := h.service.GetMetricsHistory(r.Context(), id, rangeDur)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, points)
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

func (h *DetailHandler) ERD(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	database := r.URL.Query().Get("database")

	erd, err := h.service.GetERD(r.Context(), id, database)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, erd)
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

func (h *DetailHandler) DropTable(w http.ResponseWriter, r *http.Request) {
	err := h.service.DropTable(
		r.Context(),
		r.PathValue("id"),
		r.URL.Query().Get("database"),
		r.PathValue("schema"),
		r.PathValue("table"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

type explainQueryInput struct {
	Database string `json:"database"`
	SQL      string `json:"sql"`
	Analyze  bool   `json:"analyze"`
}

func (h *DetailHandler) Explain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var in explainQueryInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if in.SQL == "" {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "campo sql é obrigatório")
		return
	}

	result, err := h.service.ExplainQuery(r.Context(), id, in.Database, in.SQL, in.Analyze)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
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

func (h *DetailHandler) LogsTimeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tail := parseIntDefault(r.URL.Query().Get("tail"), 200, 1, 2000)

	lines, err := h.service.LogsTimeline(r.Context(), id, tail)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, lines)
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
