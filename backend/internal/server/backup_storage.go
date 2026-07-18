package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// backupsBaseDir é o mount point do volume nomeado de backups (ver
// docker-compose.yml) — sempre esse caminho fixo dentro do container do
// backend, não precisa ser configurável.
const backupsBaseDir = "/backups"

// BackupStorage abstrai onde o arquivo de dump vive depois de gerado — local
// (volume Docker) ou Google Drive. RunPgDump sempre escreve primeiro num
// arquivo temporário local (pg_dump não sabe fazer upload direto), e Store
// move esse arquivo pro destino final, devolvendo uma referência que
// Open/Delete usam depois pra achar o arquivo de novo.
type BackupStorage interface {
	// Store consome (e sempre remove, sucesso ou falha) o arquivo em
	// localPath, devolvendo uma referência de armazenamento e o tamanho final.
	Store(ctx context.Context, serverID, filename, localPath string) (ref string, sizeBytes int64, err error)
	// Open devolve um caminho local pro arquivo (baixando primeiro se for
	// remoto) e uma função de limpeza pra chamar depois de usar.
	Open(ctx context.Context, ref string) (path string, cleanup func(), err error)
	Delete(ctx context.Context, ref string) error
}

type LocalStorage struct{}

func (LocalStorage) Store(ctx context.Context, serverID, filename, localPath string) (string, int64, error) {
	dir := filepath.Join(backupsBaseDir, serverID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		os.Remove(localPath)
		return "", 0, fmt.Errorf("criando diretório de backup: %w", err)
	}
	dest := filepath.Join(dir, filename)
	if err := os.Rename(localPath, dest); err != nil {
		os.Remove(localPath)
		return "", 0, fmt.Errorf("movendo backup pro destino final: %w", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		return "", 0, fmt.Errorf("lendo tamanho do backup: %w", err)
	}
	ref := filepath.Join(serverID, filename)
	return ref, info.Size(), nil
}

func (LocalStorage) Open(ctx context.Context, ref string) (string, func(), error) {
	path := filepath.Join(backupsBaseDir, ref)
	if _, err := os.Stat(path); err != nil {
		return "", nil, fmt.Errorf("arquivo de backup não encontrado: %w", err)
	}
	return path, func() {}, nil
}

func (LocalStorage) Delete(ctx context.Context, ref string) error {
	path := filepath.Join(backupsBaseDir, ref)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("apagando arquivo de backup: %w", err)
	}
	return nil
}

// newScratchFile cria o arquivo temporário onde pg_dump escreve antes de
// Store mover/subir pro destino final — sempre dentro do mesmo volume de
// backups (não /tmp do container, que costuma ser bem menor), pra um
// os.Rename local nunca precisar copiar entre filesystems diferentes.
func newScratchFile() (string, error) {
	dir := filepath.Join(backupsBaseDir, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("criando diretório temporário de backup: %w", err)
	}
	f, err := os.CreateTemp(dir, "dump-*.pgdump")
	if err != nil {
		return "", fmt.Errorf("criando arquivo temporário de backup: %w", err)
	}
	path := f.Name()
	f.Close()
	return path, nil
}
