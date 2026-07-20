package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

func (h *InfraHandler) ContainerDetail(w http.ResponseWriter, r *http.Request) {
	detail, err := h.service.ContainerDetail(r.Context(), r.PathValue("containerId"))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

func (h *InfraHandler) ContainerStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.ContainerStats(r.Context(), r.PathValue("containerId"))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

func (h *InfraHandler) ContainerStatsHistory(w http.ResponseWriter, r *http.Request) {
	history := h.service.ContainerStatsHistory(r.Context(), r.PathValue("containerId"))
	httpx.WriteJSON(w, http.StatusOK, history)
}

type connectNetworkInput struct {
	Network string `json:"network"`
}

func (h *InfraHandler) ConnectContainerNetwork(w http.ResponseWriter, r *http.Request) {
	var in connectNetworkInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.ConnectContainerNetwork(r.Context(), r.PathValue("containerId"), in.Network); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *InfraHandler) DisconnectContainerNetwork(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DisconnectContainerNetwork(r.Context(), r.PathValue("containerId"), r.PathValue("networkName")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updateResourcesInput struct {
	CPUCores float64 `json:"cpu_cores"`
	MemoryMB int     `json:"memory_mb"`
}

func (h *InfraHandler) UpdateContainerResources(w http.ResponseWriter, r *http.Request) {
	var in updateResourcesInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.UpdateContainerResources(r.Context(), r.PathValue("containerId"), in.CPUCores, in.MemoryMB); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *InfraHandler) AttachContainerVolume(w http.ResponseWriter, r *http.Request) {
	var in infra.AttachVolumeInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	newID, err := h.service.AttachVolumeToContainer(r.Context(), r.PathValue("containerId"), in)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"id": newID})
}

func (h *InfraHandler) SystemPrune(w http.ResponseWriter, r *http.Request) {
	log, err := h.service.SystemPrune(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"log": log})
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

type deployComposeInput struct {
	Name    string `json:"name"`
	Compose string `json:"compose"`
}

func (h *InfraHandler) DeployCompose(w http.ResponseWriter, r *http.Request) {
	var in deployComposeInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	project, err := h.service.DeployCompose(r.Context(), in.Name, in.Compose)
	if err != nil {
		if project != nil {
			httpx.WriteJSON(w, http.StatusUnprocessableEntity, project)
			return
		}
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, project)
}

func (h *InfraHandler) ListComposeProjects(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListComposeProjects(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *InfraHandler) RemoveComposeProject(w http.ResponseWriter, r *http.Request) {
	removeVolumes := r.URL.Query().Get("remove_volumes") == "true"
	if err := h.service.RemoveComposeProject(r.Context(), r.PathValue("name"), removeVolumes); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type buildFromDockerfileInput struct {
	Tag        string `json:"tag"`
	Dockerfile string `json:"dockerfile"`
}

func (h *InfraHandler) BuildFromDockerfile(w http.ResponseWriter, r *http.Request) {
	var in buildFromDockerfileInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	result, err := h.service.BuildFromDockerfile(r.Context(), in.Tag, in.Dockerfile)
	if err != nil {
		if result != nil {
			httpx.WriteJSON(w, http.StatusUnprocessableEntity, result)
			return
		}
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

// BuildFromContext recebe um upload multipart: campo "tag" + campo "context"
// (arquivo .tar ou .tar.gz com o contexto de build inteiro, Dockerfile
// incluso na raiz).
func (h *InfraHandler) BuildFromContext(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(200 << 20); err != nil { // 200MB de limite
		httpx.WriteError(w, http.StatusBadRequest, "upload inválido ou grande demais: "+err.Error())
		return
	}
	tag := r.FormValue("tag")
	file, header, err := r.FormFile("context")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "arquivo de contexto (\"context\") é obrigatório: "+err.Error())
		return
	}
	defer file.Close()

	gzipped := strings.HasSuffix(header.Filename, ".gz") || strings.HasSuffix(header.Filename, ".tgz")

	result, err := h.service.BuildFromContext(r.Context(), tag, file, gzipped)
	if err != nil {
		if result != nil {
			httpx.WriteJSON(w, http.StatusUnprocessableEntity, result)
			return
		}
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *InfraHandler) TraefikStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.TraefikStatus(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, status)
}

type enableTraefikInput struct {
	AcmeEmail string `json:"acme_email"`
}

func (h *InfraHandler) EnableTraefik(w http.ResponseWriter, r *http.Request) {
	var in enableTraefikInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	status, err := h.service.EnableTraefik(r.Context(), in.AcmeEmail)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, status)
}

func (h *InfraHandler) DisableTraefik(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DisableTraefik(r.Context()); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *InfraHandler) ListProxyRoutes(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListProxyRoutes(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *InfraHandler) CreateProxyRoute(w http.ResponseWriter, r *http.Request) {
	var in infra.CreateProxyRouteInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	route, err := h.service.CreateProxyRoute(r.Context(), in)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, route)
}

func (h *InfraHandler) DeleteProxyRoute(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteProxyRoute(r.Context(), r.PathValue("routeId")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *InfraHandler) ListFirewallRules(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListFirewallRules(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *InfraHandler) AddFirewallRule(w http.ResponseWriter, r *http.Request) {
	var in infra.AddFirewallRuleInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.AddFirewallRule(r.Context(), in); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *InfraHandler) RemoveFirewallRule(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.PathValue("port"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "porta inválida")
		return
	}
	if err := h.service.RemoveFirewallRule(r.Context(), port, r.PathValue("proto"), r.URL.Query().Get("from")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *InfraHandler) ListGitCredentials(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListGitCredentials(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *InfraHandler) CreateGitCredential(w http.ResponseWriter, r *http.Request) {
	var in infra.CreateGitCredentialInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	cred, err := h.service.CreateGitCredential(r.Context(), in)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, cred)
}

func (h *InfraHandler) DeleteGitCredential(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteGitCredential(r.Context(), r.PathValue("credentialId")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createFromGitResponse struct {
	ID    string             `json:"id"`
	Build *infra.BuildResult `json:"build,omitempty"`
}

func (h *InfraHandler) CreateContainerFromGit(w http.ResponseWriter, r *http.Request) {
	var in infra.CreateContainerFromGitInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	id, build, err := h.service.CreateContainerFromGit(r.Context(), in)
	if err != nil {
		if build != nil {
			httpx.WriteJSON(w, http.StatusUnprocessableEntity, createFromGitResponse{Build: build})
			return
		}
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, createFromGitResponse{ID: id, Build: build})
}
