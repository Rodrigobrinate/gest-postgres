package api

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/infra"
)

// FilesHandler serve o gerenciador de arquivos dentro de container/volume —
// nunca do host (isso é HostFilesHandler, ver host_files.go). Toda operação
// de leitura/listagem passa pela API de archive do Docker (sem exec);
// exclusão é a única exceção documentada (ver internal/infra/container_files.go).
type FilesHandler struct {
	service *infra.Service
}

func NewFilesHandler(service *infra.Service) *FilesHandler {
	return &FilesHandler{service: service}
}

func queryPath(r *http.Request) string {
	if p := r.URL.Query().Get("path"); p != "" {
		return p
	}
	return "/"
}

type writeFileInput struct {
	Content string `json:"content"`
}

// streamDownload serve o conteúdo já lido da API de archive do Docker.
// Pasta vira download comprimido (.tar.gz) — não existe ação de "comprimir"
// separada, baixar uma pasta já entrega ela compactada. Arquivo único é
// desembrulhado do tar de 1 entrada e servido cru.
func streamDownload(w http.ResponseWriter, reader io.Reader, name string, isDir bool, size int64) {
	if name == "" {
		name = "download"
	}
	if isDir {
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar.gz"`, name))
		gz := gzip.NewWriter(w)
		defer gz.Close()
		if _, err := io.Copy(gz, reader); err != nil {
			slog.Error("falha comprimindo download", "error", err)
		}
		return
	}

	tr := tar.NewReader(reader)
	if _, err := tr.Next(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "falha lendo arquivo pra download")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	if _, err := io.Copy(w, tr); err != nil {
		slog.Error("falha enviando download", "error", err)
	}
}

// --- container ---

func (h *FilesHandler) ListContainerFiles(w http.ResponseWriter, r *http.Request) {
	entries, err := h.service.ListContainerDirectory(r.Context(), r.PathValue("containerId"), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entries)
}

func (h *FilesHandler) StatContainerFile(w http.ResponseWriter, r *http.Request) {
	entry, err := h.service.StatContainerPath(r.Context(), r.PathValue("containerId"), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entry)
}

func (h *FilesHandler) ReadContainerFile(w http.ResponseWriter, r *http.Request) {
	content, err := h.service.ReadContainerFile(r.Context(), r.PathValue("containerId"), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"content": string(content)})
}

func (h *FilesHandler) WriteContainerFile(w http.ResponseWriter, r *http.Request) {
	var in writeFileInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.WriteContainerFile(r.Context(), r.PathValue("containerId"), queryPath(r), []byte(in.Content)); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FilesHandler) UploadContainerFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "upload inválido ou grande demais: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "arquivo (\"file\") é obrigatório: "+err.Error())
		return
	}
	defer file.Close()

	if err := h.service.UploadContainerPath(r.Context(), r.PathValue("containerId"), queryPath(r), header.Filename, file, header.Size); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FilesHandler) DownloadContainerFile(w http.ResponseWriter, r *http.Request) {
	p := queryPath(r)
	stat, err := h.service.StatContainerPath(r.Context(), r.PathValue("containerId"), p)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	reader, _, err := h.service.DownloadContainerPath(r.Context(), r.PathValue("containerId"), p)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	defer reader.Close()
	streamDownload(w, reader, stat.Name, stat.IsDir, stat.Size)
}

func (h *FilesHandler) DeleteContainerFile(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteContainerPath(r.Context(), r.PathValue("containerId"), queryPath(r)); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- volume ---

func (h *FilesHandler) ListVolumeFiles(w http.ResponseWriter, r *http.Request) {
	entries, err := h.service.ListVolumeDirectory(r.Context(), r.PathValue("volumeName"), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entries)
}

func (h *FilesHandler) StatVolumeFile(w http.ResponseWriter, r *http.Request) {
	entry, err := h.service.StatVolumePath(r.Context(), r.PathValue("volumeName"), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entry)
}

func (h *FilesHandler) ReadVolumeFile(w http.ResponseWriter, r *http.Request) {
	content, err := h.service.ReadVolumeFile(r.Context(), r.PathValue("volumeName"), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"content": string(content)})
}

func (h *FilesHandler) WriteVolumeFile(w http.ResponseWriter, r *http.Request) {
	var in writeFileInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.WriteVolumeFile(r.Context(), r.PathValue("volumeName"), queryPath(r), []byte(in.Content)); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FilesHandler) UploadVolumeFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "upload inválido ou grande demais: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "arquivo (\"file\") é obrigatório: "+err.Error())
		return
	}
	defer file.Close()

	if err := h.service.UploadVolumePath(r.Context(), r.PathValue("volumeName"), queryPath(r), header.Filename, file, header.Size); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FilesHandler) DownloadVolumeFile(w http.ResponseWriter, r *http.Request) {
	p := queryPath(r)
	stat, err := h.service.StatVolumePath(r.Context(), r.PathValue("volumeName"), p)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	reader, _, cleanup, err := h.service.DownloadVolumePath(r.Context(), r.PathValue("volumeName"), p)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	defer cleanup()
	defer reader.Close()
	streamDownload(w, reader, stat.Name, stat.IsDir, stat.Size)
}

func (h *FilesHandler) DeleteVolumeFile(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteVolumePath(r.Context(), r.PathValue("volumeName"), queryPath(r)); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
