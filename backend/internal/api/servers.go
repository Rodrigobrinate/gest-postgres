package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

type ServersHandler struct {
	service *server.Service
}

func NewServersHandler(service *server.Service) *ServersHandler {
	return &ServersHandler{service: service}
}

func (h *ServersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in server.CreateInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}

	created, err := h.service.Create(r.Context(), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, created)
}

func (h *ServersHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if list == nil {
		list = []*server.Server{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *ServersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, found)
}

func (h *ServersHandler) Update(w http.ResponseWriter, r *http.Request) {
	var in server.UpdateServerInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	updated, err := h.service.UpdateServer(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (h *ServersHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.Start(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *ServersHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.Stop(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *ServersHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.Restart(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

func (h *ServersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	keepVolume := r.URL.Query().Get("keep_volume") == "true"

	if err := h.service.Delete(r.Context(), id, keepVolume); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, server.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, server.ErrValidation):
		httpx.WriteError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		slog.Error("erro interno no handler de servers", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "erro interno")
	}
}
