package api

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/infra"
)

// HostFilesHandler serve o gerenciador de arquivos do HOST (aba "Arquivos
// do host") — filesystem real, dentro da raiz fixa configurada via
// HOST_FILES_ROOT (bind mount, ver docker-compose.yml). Nunca confundir com
// FilesHandler (container/volume, via API de archive do Docker) — são
// mecanismos completamente diferentes por baixo.
type HostFilesHandler struct {
	service *infra.Service
}

func NewHostFilesHandler(service *infra.Service) *HostFilesHandler {
	return &HostFilesHandler{service: service}
}

func (h *HostFilesHandler) List(w http.ResponseWriter, r *http.Request) {
	entries, err := h.service.ListHostDirectory(r.Context(), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entries)
}

func (h *HostFilesHandler) Stat(w http.ResponseWriter, r *http.Request) {
	entry, err := h.service.StatHostPath(r.Context(), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entry)
}

func (h *HostFilesHandler) Read(w http.ResponseWriter, r *http.Request) {
	content, err := h.service.ReadHostFile(r.Context(), queryPath(r))
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"content": string(content)})
}

func (h *HostFilesHandler) Write(w http.ResponseWriter, r *http.Request) {
	var in writeFileInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.WriteHostFile(r.Context(), queryPath(r), []byte(in.Content)); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HostFilesHandler) Upload(w http.ResponseWriter, r *http.Request) {
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

	if err := h.service.UploadHostFile(r.Context(), queryPath(r), header.Filename, file); err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HostFilesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteHostPath(r.Context(), queryPath(r)); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Download serve arquivo cru, ou zipa uma pasta na hora (streaming, sem
// gravar o .zip em disco) — é a "compressão" que a tela pede, embutida no
// próprio download, sem uma ação de "comprimir" separada.
func (h *HostFilesHandler) Download(w http.ResponseWriter, r *http.Request) {
	userPath := queryPath(r)
	entry, err := h.service.StatHostPath(r.Context(), userPath)
	if err != nil {
		writeInfraError(w, err)
		return
	}
	fullPath, err := h.service.ResolvedHostPath(r.Context(), userPath)
	if err != nil {
		writeInfraError(w, err)
		return
	}

	if !entry.IsDir {
		f, err := os.Open(fullPath)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "falha abrindo arquivo")
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, entry.Name))
		w.Header().Set("Content-Length", strconv.FormatInt(entry.Size, 10))
		if _, err := io.Copy(w, f); err != nil {
			slog.Error("falha enviando download do host", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, entry.Name))
	zw := zip.NewWriter(w)
	defer zw.Close()

	err = filepath.Walk(fullPath, func(walkPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(fullPath, walkPath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// filepath.Walk usa Lstat — um symlink aparece aqui como entrada
		// não-dir, e os.Open (abaixo) segue o alvo dele normalmente. Sem
		// pular symlink/arquivo não-regular aqui, um symlink plantado
		// dentro da pasta gerenciada (ex: -> /etc/shadow) sai no zip do
		// download de pasta, burlando o confinamento que a leitura de
		// arquivo único já aplica via resolveHostPath (achado de auditoria).
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil
		}
		zf, err := zw.Create(filepath.ToSlash(filepath.Join(entry.Name, rel)))
		if err != nil {
			return err
		}
		f, err := os.Open(walkPath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(zf, f)
		return err
	})
	if err != nil {
		slog.Error("falha zipando pasta do host", "error", err, "path", fullPath)
	}
}
