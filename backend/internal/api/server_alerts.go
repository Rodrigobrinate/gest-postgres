package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

func (h *DetailHandler) ListAlertRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListAlertRules(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if rules == nil {
		rules = []server.AlertRule{}
	}
	httpx.WriteJSON(w, http.StatusOK, rules)
}

func (h *DetailHandler) CreateAlertRule(w http.ResponseWriter, r *http.Request) {
	var in server.CreateAlertRuleInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	rule, err := h.service.CreateAlertRule(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rule)
}

func (h *DetailHandler) DeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	err := h.service.DeleteAlertRule(r.Context(), r.PathValue("id"), r.PathValue("ruleId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type setAlertRuleEnabledInput struct {
	Enabled bool `json:"enabled"`
}

func (h *DetailHandler) SetAlertRuleEnabled(w http.ResponseWriter, r *http.Request) {
	var in setAlertRuleEnabledInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	err := h.service.SetAlertRuleEnabled(r.Context(), r.PathValue("id"), r.PathValue("ruleId"), in.Enabled)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
