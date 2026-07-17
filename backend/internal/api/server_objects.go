package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

// ---------- Views ----------

func (h *DetailHandler) ListViews(w http.ResponseWriter, r *http.Request) {
	views, err := h.service.ListViews(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if views == nil {
		views = []server.ViewInfo{}
	}
	httpx.WriteJSON(w, http.StatusOK, views)
}

type createViewInput struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Query  string `json:"query"`
}

func (h *DetailHandler) CreateView(w http.ResponseWriter, r *http.Request) {
	var in createViewInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateView(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), in.Schema, in.Name, in.Query); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropView(w http.ResponseWriter, r *http.Request) {
	err := h.service.DropView(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), r.PathValue("schema"), r.PathValue("name"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Materialized Views ----------

func (h *DetailHandler) ListMaterializedViews(w http.ResponseWriter, r *http.Request) {
	views, err := h.service.ListMaterializedViews(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if views == nil {
		views = []server.MaterializedViewInfo{}
	}
	httpx.WriteJSON(w, http.StatusOK, views)
}

func (h *DetailHandler) CreateMaterializedView(w http.ResponseWriter, r *http.Request) {
	var in createViewInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateMaterializedView(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), in.Schema, in.Name, in.Query); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) RefreshMaterializedView(w http.ResponseWriter, r *http.Request) {
	concurrently := r.URL.Query().Get("concurrently") == "true"
	err := h.service.RefreshMaterializedView(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), r.PathValue("schema"), r.PathValue("name"), concurrently)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropMaterializedView(w http.ResponseWriter, r *http.Request) {
	err := h.service.DropMaterializedView(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), r.PathValue("schema"), r.PathValue("name"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Sequences ----------

func (h *DetailHandler) ListSequences(w http.ResponseWriter, r *http.Request) {
	seqs, err := h.service.ListSequences(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if seqs == nil {
		seqs = []server.SequenceInfo{}
	}
	httpx.WriteJSON(w, http.StatusOK, seqs)
}

func (h *DetailHandler) CreateSequence(w http.ResponseWriter, r *http.Request) {
	var in server.CreateSequenceInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateSequence(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), in); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropSequence(w http.ResponseWriter, r *http.Request) {
	err := h.service.DropSequence(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), r.PathValue("schema"), r.PathValue("name"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Types / Domains ----------

func (h *DetailHandler) ListTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.service.ListTypes(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if types == nil {
		types = []server.TypeInfo{}
	}
	httpx.WriteJSON(w, http.StatusOK, types)
}

type createEnumInput struct {
	Schema string   `json:"schema"`
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func (h *DetailHandler) CreateEnumType(w http.ResponseWriter, r *http.Request) {
	var in createEnumInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateEnumType(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), in.Schema, in.Name, in.Values); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

type createDomainInput struct {
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	BaseType  string `json:"base_type"`
	CheckExpr string `json:"check_expr"`
}

func (h *DetailHandler) CreateDomain(w http.ResponseWriter, r *http.Request) {
	var in createDomainInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateDomain(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), in.Schema, in.Name, in.BaseType, in.CheckExpr); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropType(w http.ResponseWriter, r *http.Request) {
	err := h.service.DropType(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), r.PathValue("schema"), r.PathValue("name"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Functions / Procedures ----------

func (h *DetailHandler) ListFunctions(w http.ResponseWriter, r *http.Request) {
	fns, err := h.service.ListFunctions(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if fns == nil {
		fns = []server.FunctionInfo{}
	}
	httpx.WriteJSON(w, http.StatusOK, fns)
}

type createFunctionInput struct {
	SQL string `json:"sql"`
}

func (h *DetailHandler) CreateFunction(w http.ResponseWriter, r *http.Request) {
	var in createFunctionInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateFunction(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), in.SQL); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropFunction(w http.ResponseWriter, r *http.Request) {
	identityArgs := r.URL.Query().Get("identity_args")
	err := h.service.DropFunction(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"), r.PathValue("schema"), r.PathValue("name"), identityArgs)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
