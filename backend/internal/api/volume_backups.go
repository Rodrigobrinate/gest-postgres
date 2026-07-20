package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/infra"
)

type VolumeBackupsHandler struct {
	service *infra.Service
}

func NewVolumeBackupsHandler(service *infra.Service) *VolumeBackupsHandler {
	return &VolumeBackupsHandler{service: service}
}

func (h *VolumeBackupsHandler) List(w http.ResponseWriter, r *http.Request) {
	backups, err := h.service.ListVolumeBackups(r.Context(), r.PathValue("volumeName"))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, backups)
}

func (h *VolumeBackupsHandler) Create(w http.ResponseWriter, r *http.Request) {
	backup, err := h.service.BackupVolume(r.Context(), r.PathValue("volumeName"))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, backup)
}

func (h *VolumeBackupsHandler) Download(w http.ResponseWriter, r *http.Request) {
	backup, path, err := h.service.DownloadVolumeBackup(r.Context(), r.PathValue("backupId"))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "falha abrindo backup")
		return
	}
	defer f.Close()
	modTime := backup.StartedAt
	if backup.CompletedAt != nil {
		modTime = *backup.CompletedAt
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, backup.Filename))
	http.ServeContent(w, r, backup.Filename, modTime, f)
}

func (h *VolumeBackupsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteVolumeBackup(r.Context(), r.PathValue("backupId")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
