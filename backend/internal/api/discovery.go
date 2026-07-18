package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

type DiscoveryHandler struct {
	service *server.Service
}

func NewDiscoveryHandler(service *server.Service) *DiscoveryHandler {
	return &DiscoveryHandler{service: service}
}

func (h *DiscoveryHandler) Discover(w http.ResponseWriter, r *http.Request) {
	found, err := h.service.DiscoverContainers(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if found == nil {
		found = []server.DiscoveredContainer{}
	}
	httpx.WriteJSON(w, http.StatusOK, found)
}

func (h *DiscoveryHandler) Register(w http.ResponseWriter, r *http.Request) {
	var in server.RegisterDiscoveredInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	record, err := h.service.RegisterDiscovered(r.Context(), r.PathValue("containerId"), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, record)
}
