package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
)

func (h *InfraHandler) CloudflareTunnelStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.CloudflareTunnelStatus(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, status)
}

type enableCloudflareTunnelInput struct {
	Token string `json:"token"`
}

func (h *InfraHandler) EnableCloudflareTunnel(w http.ResponseWriter, r *http.Request) {
	var in enableCloudflareTunnelInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	status, err := h.service.EnableCloudflareTunnel(r.Context(), in.Token)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, status)
}

func (h *InfraHandler) DisableCloudflareTunnel(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DisableCloudflareTunnel(r.Context()); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
