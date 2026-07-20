package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

// NotificationChannelsHandler é global (plataforma inteira), não por
// servidor — diferente do resto de server_alerts.go, que é sempre
// aninhado em /servers/{id}/... .
type NotificationChannelsHandler struct {
	service *server.Service
}

func NewNotificationChannelsHandler(service *server.Service) *NotificationChannelsHandler {
	return &NotificationChannelsHandler{service: service}
}

func (h *NotificationChannelsHandler) List(w http.ResponseWriter, r *http.Request) {
	channels, err := h.service.ListNotificationChannels(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, channels)
}

func (h *NotificationChannelsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in server.CreateNotificationChannelInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	channel, err := h.service.CreateNotificationChannel(r.Context(), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, channel)
}

func (h *NotificationChannelsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteNotificationChannel(r.Context(), r.PathValue("channelId")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationChannelsHandler) Test(w http.ResponseWriter, r *http.Request) {
	if err := h.service.TestNotificationChannel(r.Context(), r.PathValue("channelId")); err != nil {
		httpx.WriteError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
