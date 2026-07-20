package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
)

// GDriveConnection é a config/estado da conta Google Drive usada pra backups
// — uma só pra plataforma inteira (ver comentário na migration 0005).
type GDriveConnection struct {
	ClientID              string
	ClientSecretEncrypted string
	RefreshTokenEncrypted string
	AccountEmail          string
	FolderID              string
	ConnectedAt           *time.Time
}

type GDriveStatus struct {
	Configured   bool       `json:"configured"` // client_id/secret já foram salvos
	Connected    bool       `json:"connected"`  // já tem refresh_token (autorizado de verdade)
	AccountEmail string     `json:"account_email,omitempty"`
	ConnectedAt  *time.Time `json:"connected_at,omitempty"`
}

func (s *Service) getGDriveConnection(ctx context.Context) (*GDriveConnection, error) {
	var c GDriveConnection
	err := s.repo.pool.QueryRow(ctx, `
		SELECT client_id, client_secret_encrypted, refresh_token_encrypted, account_email, folder_id, connected_at
		FROM gdrive_connection WHERE id = 1
	`).Scan(&c.ClientID, &c.ClientSecretEncrypted, &c.RefreshTokenEncrypted, &c.AccountEmail, &c.FolderID, &c.ConnectedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &GDriveConnection{}, nil
		}
		return nil, fmt.Errorf("lendo conexão do google drive: %w", err)
	}
	return &c, nil
}

func (s *Service) GDriveStatus(ctx context.Context) (*GDriveStatus, error) {
	c, err := s.getGDriveConnection(ctx)
	if err != nil {
		return nil, err
	}
	return &GDriveStatus{
		Configured:   c.ClientID != "",
		Connected:    c.RefreshTokenEncrypted != "",
		AccountEmail: c.AccountEmail,
		ConnectedAt:  c.ConnectedAt,
	}, nil
}

type SetGDriveConfigInput struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// SetGDriveConfig salva o client_id/secret do app OAuth que o próprio usuário
// registra no Google Cloud Console — a plataforma não tem (nem poderia ter)
// um app Google próprio embutido, cada instalação usa o app do seu dono.
func (s *Service) SetGDriveConfig(ctx context.Context, in SetGDriveConfigInput) error {
	if in.ClientID == "" || in.ClientSecret == "" {
		return fmt.Errorf("%w: client_id e client_secret são obrigatórios", ErrValidation)
	}
	encrypted, err := s.secretBox.Seal(in.ClientSecret)
	if err != nil {
		return fmt.Errorf("cifrando client secret: %w", err)
	}
	_, err = s.repo.pool.Exec(ctx, `
		INSERT INTO gdrive_connection (id, client_id, client_secret_encrypted)
		VALUES (1, $1, $2)
		ON CONFLICT (id) DO UPDATE SET client_id = $1, client_secret_encrypted = $2, updated_at = now()
	`, in.ClientID, encrypted)
	if err != nil {
		return fmt.Errorf("salvando config do google drive: %w", err)
	}
	return nil
}

func (s *Service) gdriveOAuthConfig(ctx context.Context, redirectURL string) (*oauth2.Config, error) {
	c, err := s.getGDriveConnection(ctx)
	if err != nil {
		return nil, err
	}
	if c.ClientID == "" {
		return nil, fmt.Errorf("%w: configure o client_id/secret do Google antes", ErrValidation)
	}
	secret, err := s.secretBox.Open(c.ClientSecretEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decifrando client secret: %w", err)
	}
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: secret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
		RedirectURL: redirectURL,
		Scopes:      []string{"https://www.googleapis.com/auth/drive.file"},
	}, nil
}

// GDriveAuthURL monta a URL de consentimento — o admin visita ela uma vez
// (fora da plataforma, no navegador dele mesmo, é a própria Google pedindo
// login, não tem como isso ser automatizado por ninguém além do dono da
// conta) pra autorizar o acesso. access_type=offline + prompt=consent
// garantem que a Google devolve um refresh_token de verdade — sem isso, numa
// reautorização ela só devolve access_token, que expira em 1h e não dá pra
// renovar sozinho depois.
//
// state é aleatório (32 bytes) e de uso único, guardado com expiração curta
// — antes era a constante fixa "gestpg", nunca validada no callback, o que
// não fornecia proteção nenhuma de CSRF (um atacante conseguia induzir um
// admin logado a vincular a CONTA GOOGLE DO ATACANTE, redirecionando todo
// backup futuro pro Drive dele).
func (s *Service) GDriveAuthURL(ctx context.Context, redirectURL string) (string, error) {
	cfg, err := s.gdriveOAuthConfig(ctx, redirectURL)
	if err != nil {
		return "", err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gerando state do oauth: %w", err)
	}
	state := hex.EncodeToString(buf)
	_, err = s.repo.pool.Exec(ctx, `
		UPDATE gdrive_connection SET oauth_state = $1, oauth_state_expires_at = now() + interval '10 minutes'
		WHERE id = 1
	`, state)
	if err != nil {
		return "", fmt.Errorf("salvando state do oauth: %w", err)
	}
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent")), nil
}

// GDriveCallback troca o code pelo token e salva o refresh_token cifrado —
// chamado pelo handler do redirect_uri configurado no Google Cloud Console.
// state precisa bater com o gerado por GDriveAuthURL e não ter expirado;
// é limpo (uso único) antes de qualquer outra coisa, sucesso ou falha.
func (s *Service) GDriveCallback(ctx context.Context, code, state, redirectURL string) error {
	var storedState string
	var expiresAt *time.Time
	err := s.repo.pool.QueryRow(ctx, `SELECT oauth_state, oauth_state_expires_at FROM gdrive_connection WHERE id = 1`).
		Scan(&storedState, &expiresAt)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("lendo state do oauth: %w", err)
	}
	_, _ = s.repo.pool.Exec(ctx, `UPDATE gdrive_connection SET oauth_state = '', oauth_state_expires_at = NULL WHERE id = 1`)

	if storedState == "" || subtle.ConstantTimeCompare([]byte(storedState), []byte(state)) != 1 {
		return fmt.Errorf("%w: state do oauth inválido ou ausente — reinicie o fluxo de autorização", ErrValidation)
	}
	if expiresAt == nil || time.Now().After(*expiresAt) {
		return fmt.Errorf("%w: state do oauth expirado — reinicie o fluxo de autorização", ErrValidation)
	}

	cfg, err := s.gdriveOAuthConfig(ctx, redirectURL)
	if err != nil {
		return err
	}
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("%w: trocando code por token: %v", ErrValidation, err)
	}
	if token.RefreshToken == "" {
		return fmt.Errorf("%w: Google não devolveu refresh_token — revogue o acesso em myaccount.google.com/permissions e tente de novo", ErrValidation)
	}

	email, _ := fetchGoogleAccountEmail(ctx, cfg, token)

	encrypted, err := s.secretBox.Seal(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("cifrando refresh token: %w", err)
	}
	_, err = s.repo.pool.Exec(ctx, `
		UPDATE gdrive_connection SET refresh_token_encrypted = $1, account_email = $2, connected_at = now(), updated_at = now()
		WHERE id = 1
	`, encrypted, email)
	if err != nil {
		return fmt.Errorf("salvando conexão do google drive: %w", err)
	}
	return nil
}

func (s *Service) GDriveDisconnect(ctx context.Context) error {
	_, err := s.repo.pool.Exec(ctx, `
		UPDATE gdrive_connection SET refresh_token_encrypted = '', account_email = '', folder_id = '', connected_at = NULL, updated_at = now()
		WHERE id = 1
	`)
	if err != nil {
		return fmt.Errorf("desconectando google drive: %w", err)
	}
	return nil
}

func fetchGoogleAccountEmail(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (string, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.Email, nil
}
