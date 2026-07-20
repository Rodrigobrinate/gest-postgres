package infra

import (
	"context"
	"fmt"
	"time"
)

type GitCredentialKind string

const (
	GitCredentialSSHKey GitCredentialKind = "ssh_key"
	GitCredentialPAT    GitCredentialKind = "pat"
)

// GitCredential é a versão SEM o segredo, devolvida por List — o secret
// cifrado nunca sai do backend depois de salvo (mesmo raciocínio de nunca
// devolver a senha decifrada de servidor gerenciado por acaso).
type GitCredential struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      GitCredentialKind `json:"kind"`
	Username  string            `json:"username,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

type CreateGitCredentialInput struct {
	Name     string            `json:"name"`
	Kind     GitCredentialKind `json:"kind"`
	Username string            `json:"username,omitempty"`
	Secret   string            `json:"secret"`
}

func (s *Service) ListGitCredentials(ctx context.Context) ([]GitCredential, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, kind, username, created_at
		FROM git_credentials
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("listando credenciais git: %w", err)
	}
	defer rows.Close()

	list := []GitCredential{}
	for rows.Next() {
		var c GitCredential
		if err := rows.Scan(&c.ID, &c.Name, &c.Kind, &c.Username, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("lendo credencial git: %w", err)
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Service) CreateGitCredential(ctx context.Context, in CreateGitCredentialInput) (*GitCredential, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("nome é obrigatório")
	}
	if in.Kind != GitCredentialSSHKey && in.Kind != GitCredentialPAT {
		return nil, fmt.Errorf("kind deve ser 'ssh_key' ou 'pat'")
	}
	if in.Secret == "" {
		return nil, fmt.Errorf("secret é obrigatório")
	}

	encrypted, err := s.secretBox.Seal(in.Secret)
	if err != nil {
		return nil, fmt.Errorf("cifrando credencial git: %w", err)
	}

	var c GitCredential
	err = s.pool.QueryRow(ctx, `
		INSERT INTO git_credentials (name, kind, username, secret_encrypted)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, name, kind, username, created_at
	`, in.Name, in.Kind, in.Username, encrypted).Scan(&c.ID, &c.Name, &c.Kind, &c.Username, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("salvando credencial git: %w", err)
	}
	return &c, nil
}

func (s *Service) DeleteGitCredential(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM git_credentials WHERE id = $1`, id); err != nil {
		return fmt.Errorf("removendo credencial git: %w", err)
	}
	return nil
}

// resolveGitCredential decifra a credencial pra uso imediato num `git
// clone` (ver git_build.go) — nunca guardada em texto puro, nunca logada.
func (s *Service) resolveGitCredential(ctx context.Context, id string) (kind GitCredentialKind, username, secret string, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT kind, username, secret_encrypted FROM git_credentials WHERE id = $1
	`, id).Scan(&kind, &username, &secret)
	if err != nil {
		return "", "", "", fmt.Errorf("lendo credencial git: %w", err)
	}
	secret, err = s.secretBox.Open(secret)
	if err != nil {
		return "", "", "", fmt.Errorf("decifrando credencial git: %w", err)
	}
	return kind, username, secret, nil
}
