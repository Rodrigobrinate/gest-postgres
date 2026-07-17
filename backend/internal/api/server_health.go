package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

func (h *DetailHandler) Bloat(w http.ResponseWriter, r *http.Request) {
	bloat, err := h.service.ListBloat(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if bloat == nil {
		bloat = []server.TableBloat{}
	}
	httpx.WriteJSON(w, http.StatusOK, bloat)
}

func (h *DetailHandler) Wraparound(w http.ResponseWriter, r *http.Request) {
	wrap, err := h.service.WraparoundStatus(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if wrap == nil {
		wrap = []server.WraparoundInfo{}
	}
	httpx.WriteJSON(w, http.StatusOK, wrap)
}

func (h *DetailHandler) HealthScore(w http.ResponseWriter, r *http.Request) {
	score, err := h.service.GetHealthScore(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, score)
}

func (h *DetailHandler) CapacityForecast(w http.ResponseWriter, r *http.Request) {
	forecast, err := h.service.GetCapacityForecast(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, forecast)
}
