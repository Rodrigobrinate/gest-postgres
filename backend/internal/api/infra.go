package api

import (
	"log/slog"
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/infra"
)

type InfraHandler struct {
	service *infra.Service
}

func NewInfraHandler(service *infra.Service) *InfraHandler {
	return &InfraHandler{service: service}
}

func writeInfraError(w http.ResponseWriter, err error) {
	slog.Error("erro no handler de infra", "error", err)
	httpx.WriteError(w, http.StatusBadRequest, err.Error())
}

func (h *InfraHandler) ListContainers(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListContainers(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *InfraHandler) CreateContainer(w http.ResponseWriter, r *http.Request) {
	var in infra.CreateContainerFromImageInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	id, err := h.service.CreateContainerFromImage(r.Context(), in)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *InfraHandler) StartContainer(w http.ResponseWriter, r *http.Request) {
	if err := h.service.StartContainer(r.Context(), r.PathValue("containerId")); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *InfraHandler) StopContainer(w http.ResponseWriter, r *http.Request) {
	if err := h.service.StopContainer(r.Context(), r.PathValue("containerId")); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *InfraHandler) RestartContainer(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RestartContainer(r.Context(), r.PathValue("containerId")); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *InfraHandler) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RemoveContainer(r.Context(), r.PathValue("containerId")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *InfraHandler) ContainerLogs(w http.ResponseWriter, r *http.Request) {
	tail := parseIntDefault(r.URL.Query().Get("tail"), 500, 1, 5000)
	logs, err := h.service.ContainerLogs(r.Context(), r.PathValue("containerId"), tail)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

func (h *InfraHandler) ListNetworks(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListNetworks(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

type createNetworkInput struct {
	Name string `json:"name"`
}

func (h *InfraHandler) CreateNetwork(w http.ResponseWriter, r *http.Request) {
	var in createNetworkInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateNetwork(r.Context(), in.Name); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *InfraHandler) RemoveNetwork(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RemoveNetwork(r.Context(), r.PathValue("networkId")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *InfraHandler) ListVolumes(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListVolumes(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

type createVolumeInput struct {
	Name string `json:"name"`
}

func (h *InfraHandler) CreateVolume(w http.ResponseWriter, r *http.Request) {
	var in createVolumeInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.CreateVolume(r.Context(), in.Name); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *InfraHandler) RemoveVolume(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RemoveVolume(r.Context(), r.PathValue("volumeName")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
