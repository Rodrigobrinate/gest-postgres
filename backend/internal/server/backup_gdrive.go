package server

import (
	"context"
	"fmt"
)

// gdriveStorage carrega a conexão Google Drive configurada (se houver) e
// devolve um BackupStorage que fala com a Drive API v3. Implementado em
// backup_gdrive_client.go — aqui só a checagem de "tá configurado".
func (s *Service) gdriveStorage(ctx context.Context) (BackupStorage, error) {
	conn, err := s.getGDriveConnection(ctx)
	if err != nil {
		return nil, err
	}
	if conn.RefreshTokenEncrypted == "" {
		return nil, fmt.Errorf("%w: Google Drive ainda não conectado — configure em Configuração > Backup", ErrValidation)
	}
	return newGoogleDriveStorage(ctx, s, conn)
}
