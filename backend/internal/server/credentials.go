package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RotateSuperuserPassword troca a senha do usuário superuser criado junto com
// o servidor — ALTER ROLE de verdade no Postgres, e o valor cifrado no
// metadata DB é atualizado no mesmo golpe (é essa cópia que alimenta connection
// string e reconexões do backend, inclusive as que o próprio backend faz pra
// tudo — monitoramento, editor SQL, todas as abas). Retorna a senha em texto
// puro uma vez só, igual acontece na criação do servidor.
//
// Ordem importa aqui: cifra a senha nova E confirma a conexão ANTES de trocar
// no Postgres, e se salvar no metadata DB falhar depois do ALTER ROLE já ter
// ido, reverte a senha no Postgres pra senha antiga em vez de deixar o
// servidor com a senha trocada mas o metadata DB apontando pra senha errada
// — isso deixaria o servidor inacessível pra sempre pela plataforma, sem
// forma de recuperar pela UI (a senha antiga funcionaria ainda, mas
// só reescrevendo o banco de metadados na mão o operador conseguiria voltar).
func (s *Service) RotateSuperuserPassword(ctx context.Context, id string) (string, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return "", err
	}

	oldPassword, err := s.secretBox.Open(record.PasswordEncrypted)
	if err != nil {
		return "", fmt.Errorf("decifrando senha atual: %w", err)
	}

	newPassword, err := generatePassword()
	if err != nil {
		return "", fmt.Errorf("gerando senha: %w", err)
	}

	// Cifra ANTES de mexer no Postgres — se isso falhar, nada mudou ainda.
	encrypted, err := s.secretBox.Seal(newPassword)
	if err != nil {
		return "", fmt.Errorf("cifrando nova senha: %w", err)
	}

	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	usernameIdent := pgx.Identifier{record.Username}.Sanitize()
	sql := fmt.Sprintf("ALTER ROLE %s WITH PASSWORD %s", usernameIdent, sqlQuoteLiteral(newPassword))
	if _, err := conn.Exec(ctx, sql); err != nil {
		return "", fmt.Errorf("%w: %v", ErrValidation, err)
	}

	if err := s.repo.UpdatePassword(ctx, id, encrypted); err != nil {
		// Postgres já trocou mas não conseguimos salvar — reverte pra senha
		// antiga em vez de deixar o metadata DB desincronizado do Postgres.
		revertSQL := fmt.Sprintf("ALTER ROLE %s WITH PASSWORD %s", usernameIdent, sqlQuoteLiteral(oldPassword))
		if _, revertErr := conn.Exec(ctx, revertSQL); revertErr != nil {
			return "", fmt.Errorf(
				"CRÍTICO: senha trocada no Postgres mas falhou ao salvar (%v) e falhou ao reverter (%v) — servidor pode estar inacessível pela plataforma, a senha antiga ainda funciona direto no Postgres",
				err, revertErr,
			)
		}
		return "", fmt.Errorf("falha ao salvar a nova senha, revertido com sucesso, nada mudou: %w", err)
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
