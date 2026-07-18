package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

type PlatformHandler struct {
	service *server.Service
}

func NewPlatformHandler(service *server.Service) *PlatformHandler {
	return &PlatformHandler{service: service}
}

func (h *PlatformHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetPlatformStats(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}
