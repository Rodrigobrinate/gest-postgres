package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

func (h *DetailHandler) ListRetentionPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := h.service.ListRetentionPolicies(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if policies == nil {
		policies = []server.RetentionPolicy{}
	}
	httpx.WriteJSON(w, http.StatusOK, policies)
}

func (h *DetailHandler) CreateRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	var in server.CreateRetentionPolicyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	policy, err := h.service.CreateRetentionPolicy(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, policy)
}

func (h *DetailHandler) DeleteRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	err := h.service.DeleteRetentionPolicy(r.Context(), r.PathValue("id"), r.PathValue("policyId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type setRetentionEnabledInput struct {
	Enabled bool `json:"enabled"`
}

func (h *DetailHandler) SetRetentionPolicyEnabled(w http.ResponseWriter, r *http.Request) {
	var in setRetentionEnabledInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	err := h.service.SetRetentionPolicyEnabled(r.Context(), r.PathValue("id"), r.PathValue("policyId"), in.Enabled)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) RunRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	rows, err := h.service.RunRetentionPolicy(r.Context(), r.PathValue("policyId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int64{"rows_affected": rows})
}
