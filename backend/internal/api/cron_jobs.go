package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/infra"
)

type CronJobsHandler struct {
	service *infra.Service
}

func NewCronJobsHandler(service *infra.Service) *CronJobsHandler {
	return &CronJobsHandler{service: service}
}

func (h *CronJobsHandler) List(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("container_id")
	jobs, err := h.service.ListCronJobs(r.Context(), containerID)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, jobs)
}

func (h *CronJobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in infra.CreateCronJobInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	job, err := h.service.CreateCronJob(r.Context(), in)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, job)
}

func (h *CronJobsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteCronJob(r.Context(), r.PathValue("cronJobId")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type setCronJobEnabledInput struct {
	Enabled bool `json:"enabled"`
}

func (h *CronJobsHandler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	var in setCronJobEnabledInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.SetCronJobEnabled(r.Context(), r.PathValue("cronJobId"), in.Enabled); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *CronJobsHandler) RunNow(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.RunCronJobNow(r.Context(), r.PathValue("cronJobId"))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, job)
}
