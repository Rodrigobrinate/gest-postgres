package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"

	"golang.org/x/oauth2"
)

// googleDriveStorage fala direto com a Drive API v3 via REST (sem o SDK
// google.golang.org/api, que puxa uma árvore de dependências bem maior do
// que precisamos pra 3 operações: upload/download/delete de um arquivo numa
// pasta fixa). golang.org/x/oauth2 cuida de renovar o access_token sozinho
// usando o refresh_token guardado, então esse client HTTP nunca precisa se
// preocupar com token expirado.
type googleDriveStorage struct {
	client   *http.Client
	folderID string
}

func newGoogleDriveStorage(ctx context.Context, s *Service, conn *GDriveConnection) (BackupStorage, error) {
	refreshToken, err := s.secretBox.Open(conn.RefreshTokenEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decifrando refresh token do google drive: %w", err)
	}
	cfg, err := s.gdriveOAuthConfig(ctx, "")
	if err != nil {
		return nil, err
	}
	client := cfg.Client(ctx, &oauth2.Token{RefreshToken: refreshToken})

	folderID := conn.FolderID
	if folderID == "" {
		folderID, err = ensureBackupsFolder(client)
		if err != nil {
			return nil, err
		}
		if _, err := s.repo.pool.Exec(ctx, `UPDATE gdrive_connection SET folder_id = $1 WHERE id = 1`, folderID); err != nil {
			return nil, fmt.Errorf("salvando pasta do google drive: %w", err)
		}
	}

	return &googleDriveStorage{client: client, folderID: folderID}, nil
}

func ensureBackupsFolder(client *http.Client) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"name":     "gest-postgres-backups",
		"mimeType": "application/vnd.google-apps.folder",
	})
	req, err := http.NewRequest(http.MethodPost, "https://www.googleapis.com/drive/v3/files", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("criando pasta no google drive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", driveError("criando pasta no google drive", resp)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// Store faz upload em streaming (io.Pipe + multipart.Writer escrevendo numa
// goroutine) pra nunca precisar carregar o dump inteiro na memória — dumps
// de banco grande podem ser vários GB.
func (g *googleDriveStorage) Store(ctx context.Context, serverID, filename, localPath string) (string, int64, error) {
	defer os.Remove(localPath)

	f, err := os.Open(localPath)
	if err != nil {
		return "", 0, fmt.Errorf("abrindo arquivo de backup: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", 0, err
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mw.Close()

		metaPart, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		meta, _ := json.Marshal(map[string]any{"name": filename, "parents": []string{g.folderID}})
		if _, err := metaPart.Write(meta); err != nil {
			pw.CloseWithError(err)
			return
		}

		filePart, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}})
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(filePart, f); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart", pr)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "multipart/related; boundary="+mw.Boundary())

	resp, err := g.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("subindo backup pro google drive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", 0, driveError("subindo backup pro google drive", resp)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, err
	}
	return out.ID, info.Size(), nil
}

func (g *googleDriveStorage) Open(ctx context.Context, ref string) (string, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/drive/v3/files/"+ref+"?alt=media", nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("baixando backup do google drive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", nil, driveError("baixando backup do google drive", resp)
	}

	tmpPath, err := newScratchFile()
	if err != nil {
		return "", nil, err
	}
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return "", nil, fmt.Errorf("gravando backup baixado: %w", err)
	}
	out.Close()

	return tmpPath, func() { os.Remove(tmpPath) }, nil
}

func (g *googleDriveStorage) Delete(ctx context.Context, ref string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "https://www.googleapis.com/drive/v3/files/"+ref, nil)
	if err != nil {
		return err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("apagando backup do google drive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return driveError("apagando backup do google drive", resp)
	}
	return nil
}

func driveError(action string, resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s: status %d: %s", action, resp.StatusCode, string(b))
}
