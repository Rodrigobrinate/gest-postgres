package api

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

type createBackupInput struct {
	Database string `json:"database"`
	Storage  string `json:"storage"`
}

func (h *DetailHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	var in createBackupInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	backup, err := h.service.CreateBackup(r.Context(), r.PathValue("id"), in.Database, in.Storage)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, backup)
}

func (h *DetailHandler) ListBackups(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListBackups(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *DetailHandler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteBackup(r.Context(), r.PathValue("id"), r.PathValue("backupId")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DetailHandler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	path, filename, cleanup, err := h.service.DownloadBackup(r.Context(), r.PathValue("id"), r.PathValue("backupId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	defer cleanup()

	f, err := os.Open(path)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "erro lendo arquivo de backup")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, f)
}

func (h *DetailHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	var in server.RestoreBackupInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.RestoreBackup(r.Context(), r.PathValue("id"), r.PathValue("backupId"), in); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) CreateBackupPolicy(w http.ResponseWriter, r *http.Request) {
	var in server.CreateBackupPolicyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	p, err := h.service.CreateBackupPolicy(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, p)
}

func (h *DetailHandler) ListBackupPolicies(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListBackupPolicies(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *DetailHandler) DeleteBackupPolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteBackupPolicy(r.Context(), r.PathValue("id"), r.PathValue("policyId")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type setBackupPolicyEnabledInput struct {
	Enabled bool `json:"enabled"`
}

func (h *DetailHandler) SetBackupPolicyEnabled(w http.ResponseWriter, r *http.Request) {
	var in setBackupPolicyEnabledInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.SetBackupPolicyEnabled(r.Context(), r.PathValue("id"), r.PathValue("policyId"), in.Enabled); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) RunBackupPolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RunBackupPolicyNow(r.Context(), r.PathValue("policyId")); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GDriveHandler cobre os endpoints da conexão Google Drive — é da
// PLATAFORMA inteira, não por servidor, por isso não tem {id} na rota nem
// vive dentro do DetailHandler.
type GDriveHandler struct {
	service *server.Service
}

func NewGDriveHandler(service *server.Service) *GDriveHandler {
	return &GDriveHandler{service: service}
}

func (h *GDriveHandler) Status(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.GDriveStatus(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, status)
}

func (h *GDriveHandler) SetConfig(w http.ResponseWriter, r *http.Request) {
	var in server.SetGDriveConfigInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.SetGDriveConfig(r.Context(), in); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// redirectURL monta a URL do próprio endpoint de callback a partir do que a
// requisição atual enxerga — assim funciona tanto local (localhost:28080)
// quanto atrás de qualquer domínio/porta pública sem precisar configurar
// isso à parte. Precisa bater exatamente com o "URI de redirecionamento
// autorizado" cadastrado no Google Cloud Console.
func redirectURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/api/v1/gdrive/callback", scheme, r.Host)
}

func (h *GDriveHandler) AuthURL(w http.ResponseWriter, r *http.Request) {
	url, err := h.service.GDriveAuthURL(r.Context(), redirectURL(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"url": url})
}

// Callback é pra onde a Google redireciona depois do usuário autorizar —
// GET direto no navegador (fluxo padrão OAuth), não uma chamada de API do
// frontend, por isso devolve HTML simples em vez de JSON.
func (h *GDriveHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<html><body><p>Autorização cancelada ou sem código. Pode fechar essa aba.</p></body></html>`)
		return
	}
	if err := h.service.GDriveCallback(r.Context(), code, redirectURL(r)); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<html><body><p>Falha conectando ao Google Drive: %s</p></body></html>`, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<html><body><p>Google Drive conectado. Pode fechar essa aba e voltar pra plataforma.</p><script>window.close()</script></body></html>`)
}

func (h *GDriveHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.service.GDriveDisconnect(r.Context()); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
