package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RotateSuperuserPassword troca a senha do usuário superuser criado junto com
// o servidor — ALTER ROLE de verdade no Postgres, e o valor cifrado no
// metadata DB é atualizado no mesmo golpe (é essa cópia que alimenta connection
// string e reconexões do backend). Retorna a senha em texto puro uma vez só,
// igual acontece na criação do servidor.
func (s *Service) RotateSuperuserPassword(ctx context.Context, id string) (string, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return "", err
	}

	newPassword, err := generatePassword()
	if err != nil {
		return "", fmt.Errorf("gerando senha: %w", err)
	}

	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	sql := fmt.Sprintf("ALTER ROLE %s WITH PASSWORD %s", pgx.Identifier{record.Username}.Sanitize(), sqlQuoteLiteral(newPassword))
	if _, err := conn.Exec(ctx, sql); err != nil {
		return "", fmt.Errorf("%w: %v", ErrValidation, err)
	}

	encrypted, err := s.secretBox.Seal(newPassword)
	if err != nil {
		return "", fmt.Errorf("cifrando nova senha: %w", err)
	}
	if err := s.repo.UpdatePassword(ctx, id, encrypted); err != nil {
		return "", fmt.Errorf("salvando nova senha: %w", err)
	}

	return newPassword, nil
}

// RotateRolePassword troca a senha de qualquer outra role (não o superuser
// do servidor) — usado em Usuários. A senha não fica guardada em lugar
// nenhum da plataforma pra roles que não são o superuser, então essa é a
// única chance de ver o valor novo.
func (s *Service) RotateRolePassword(ctx context.Context, id, roleName string) (string, error) {
	if !identRegex.MatchString(roleName) {
		return "", fmt.Errorf("%w: nome de role inválido", ErrValidation)
	}
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return "", err
	}

	newPassword, err := generatePassword()
	if err != nil {
		return "", fmt.Errorf("gerando senha: %w", err)
	}

	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	sql := fmt.Sprintf("ALTER ROLE %s WITH PASSWORD %s", pgx.Identifier{roleName}.Sanitize(), sqlQuoteLiteral(newPassword))
	if _, err := conn.Exec(ctx, sql); err != nil {
		return "", fmt.Errorf("%w: %v", ErrValidation, err)
	}

	return newPassword, nil
}
