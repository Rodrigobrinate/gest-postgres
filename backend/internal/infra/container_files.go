package infra

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/gest-postgres/backend/internal/docker"
)

// ownPlatformServices são os serviços do NOSSO próprio stack (mesmo mapa da
// auto-descoberta, ver server/discovery.go) — nunca alvo do file manager de
// container. Sem essa checagem, uma sessão com acesso de leitura ao file
// manager lê /proc/1/environ do container do BACKEND, que carrega
// CREDENTIAL_ENCRYPTION_KEY — decifra todo segredo guardado na plataforma.
var ownPlatformServices = map[string]bool{
	"metadata-db":         true,
	"docker-socket-proxy": true,
	"backend":             true,
	"frontend":            true,
}

func (s *Service) guardNotOwnContainer(ctx context.Context, containerID string) error {
	detail, err := s.docker.InspectContainerFull(ctx, containerID)
	if err != nil {
		return err
	}
	if ownPlatformServices[detail.Labels["com.docker.compose.service"]] {
		return fmt.Errorf("gerenciamento de arquivo bloqueado nos containers da própria plataforma")
	}
	return nil
}

func validatePath(p string) (string, error) {
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("caminho deve ser absoluto")
	}
	clean := path.Clean(p)
	// /proc expõe o ambiente/memória do processo vivo do container (ex:
	// /proc/1/environ) — nunca faz sentido navegar ali num file manager de
	// "arquivo", só serve pra vazar segredo de env var.
	if clean == "/proc" || strings.HasPrefix(clean, "/proc/") {
		return "", fmt.Errorf("/proc não é acessível pelo gerenciador de arquivos")
	}
	return clean, nil
}

// validateMutablePath é mais estrito — usado só pra escrita/upload/exclusão,
// nunca deixa a operação mirar a raiz do filesystem inteira.
func validateMutablePath(p string) (string, error) {
	clean, err := validatePath(p)
	if err != nil {
		return "", err
	}
	if clean == "/" {
		return "", fmt.Errorf("não é permitido operar na raiz do filesystem")
	}
	return clean, nil
}

func validateFilename(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("nome de arquivo inválido")
	}
	return nil
}

// --- container ---

func (s *Service) ListContainerDirectory(ctx context.Context, containerID, dirPath string) ([]docker.FileEntry, error) {
	if err := s.guardNotOwnContainer(ctx, containerID); err != nil {
		return nil, err
	}
	dirPath, err := validatePath(dirPath)
	if err != nil {
		return nil, err
	}
	return s.docker.ListDirectoryInContainer(ctx, containerID, dirPath)
}

func (s *Service) ReadContainerFile(ctx context.Context, containerID, filePath string) ([]byte, error) {
	if err := s.guardNotOwnContainer(ctx, containerID); err != nil {
		return nil, err
	}
	filePath, err := validatePath(filePath)
	if err != nil {
		return nil, err
	}
	return s.docker.ReadFileFromContainer(ctx, containerID, filePath)
}

func (s *Service) WriteContainerFile(ctx context.Context, containerID, filePath string, content []byte) error {
	if err := s.guardNotOwnContainer(ctx, containerID); err != nil {
		return err
	}
	filePath, err := validateMutablePath(filePath)
	if err != nil {
		return err
	}
	return s.docker.WriteFileToContainer(ctx, containerID, filePath, content, 0o644)
}

func (s *Service) UploadContainerPath(ctx context.Context, containerID, destDir, filename string, content io.Reader, size int64) error {
	if err := s.guardNotOwnContainer(ctx, containerID); err != nil {
		return err
	}
	destDir, err := validatePath(destDir)
	if err != nil {
		return err
	}
	if err := validateFilename(filename); err != nil {
		return err
	}
	return s.docker.UploadFileToContainer(ctx, containerID, destDir, filename, content, size, 0o644)
}

func (s *Service) DownloadContainerPath(ctx context.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
	if err := s.guardNotOwnContainer(ctx, containerID); err != nil {
		return nil, types.ContainerPathStat{}, err
	}
	srcPath, err := validatePath(srcPath)
	if err != nil {
		return nil, types.ContainerPathStat{}, err
	}
	return s.docker.DownloadFromContainer(ctx, containerID, srcPath)
}

// StatContainerPath consulta propriedades (tamanho, permissão, data) de um
// caminho sem baixar o conteúdo — usado pela tela de "propriedades" do
// file manager.
func (s *Service) StatContainerPath(ctx context.Context, containerID, targetPath string) (docker.FileEntry, error) {
	if err := s.guardNotOwnContainer(ctx, containerID); err != nil {
		return docker.FileEntry{}, err
	}
	targetPath, err := validatePath(targetPath)
	if err != nil {
		return docker.FileEntry{}, err
	}
	stat, err := s.docker.StatPathInContainer(ctx, containerID, targetPath)
	if err != nil {
		return docker.FileEntry{}, err
	}
	return docker.FileEntry{
		Name:    stat.Name,
		Path:    targetPath,
		IsDir:   stat.Mode.IsDir(),
		Size:    stat.Size,
		Mode:    int64(stat.Mode.Perm()),
		ModTime: stat.Mtime,
	}, nil
}

func (s *Service) DeleteContainerPath(ctx context.Context, containerID, targetPath string) error {
	if err := s.guardNotOwnContainer(ctx, containerID); err != nil {
		return err
	}
	targetPath, err := validateMutablePath(targetPath)
	if err != nil {
		return err
	}
	return s.docker.DeleteInContainer(ctx, containerID, targetPath)
}

// --- volume (via container auxiliar efêmero) ---

const (
	fileHelperImage  = "alpine:3.21"
	volumeMountPoint = "/vol"
)

// withVolumeHelper sobe um container alpine descartável com o volume
// montado em /vol, roda fn contra ele, e sempre remove no final (best
// effort, com contexto próprio pra não depender do request original ainda
// estar vivo). É a mesma tática pros arquivos DENTRO de um volume nomeado
// que já usamos pra container — a API de archive do Docker só enxerga
// filesystem de container, não volume solto, então "gerenciar arquivo do
// volume" aqui é sempre "gerenciar arquivo de um container que tem esse
// volume montado", só que descartável.
func (s *Service) withVolumeHelper(ctx context.Context, volumeName string, fn func(helperID string) error) error {
	if err := validateVolumeName(volumeName); err != nil {
		return err
	}
	if err := s.docker.PullImageIfMissing(ctx, fileHelperImage); err != nil {
		return err
	}
	helperID, err := s.docker.CreateGenericContainer(ctx, docker.CreateGenericContainerInput{
		Image:   fileHelperImage,
		Command: []string{"sleep", "300"},
		Binds:   []string{volumeName + ":" + volumeMountPoint},
	})
	if err != nil {
		return fmt.Errorf("preparando acesso ao volume %s: %w", volumeName, err)
	}
	defer func() {
		_ = s.docker.RemoveContainer(context.Background(), helperID, "", false)
	}()

	return fn(helperID)
}

func volumeInternalPath(sub string) (string, error) {
	sub, err := validatePath(sub)
	if err != nil {
		return "", err
	}
	return path.Join(volumeMountPoint, sub), nil
}

func (s *Service) ListVolumeDirectory(ctx context.Context, volumeName, dirPath string) ([]docker.FileEntry, error) {
	full, err := volumeInternalPath(dirPath)
	if err != nil {
		return nil, err
	}
	var entries []docker.FileEntry
	err = s.withVolumeHelper(ctx, volumeName, func(helperID string) error {
		var innerErr error
		entries, innerErr = s.docker.ListDirectoryInContainer(ctx, helperID, full)
		return innerErr
	})
	return entries, err
}

func (s *Service) ReadVolumeFile(ctx context.Context, volumeName, filePath string) ([]byte, error) {
	full, err := volumeInternalPath(filePath)
	if err != nil {
		return nil, err
	}
	var content []byte
	err = s.withVolumeHelper(ctx, volumeName, func(helperID string) error {
		var innerErr error
		content, innerErr = s.docker.ReadFileFromContainer(ctx, helperID, full)
		return innerErr
	})
	return content, err
}

func (s *Service) WriteVolumeFile(ctx context.Context, volumeName, filePath string, content []byte) error {
	full, err := volumeInternalPath(filePath)
	if err != nil {
		return err
	}
	if full == volumeMountPoint {
		return fmt.Errorf("não é permitido operar na raiz do volume")
	}
	return s.withVolumeHelper(ctx, volumeName, func(helperID string) error {
		return s.docker.WriteFileToContainer(ctx, helperID, full, content, 0o644)
	})
}

func (s *Service) UploadVolumePath(ctx context.Context, volumeName, destDir, filename string, content io.Reader, size int64) error {
	full, err := volumeInternalPath(destDir)
	if err != nil {
		return err
	}
	if err := validateFilename(filename); err != nil {
		return err
	}
	return s.withVolumeHelper(ctx, volumeName, func(helperID string) error {
		return s.docker.UploadFileToContainer(ctx, helperID, full, filename, content, size, 0o644)
	})
}

// DownloadVolumePath não usa o padrão withVolumeHelper porque o download
// precisa manter o stream aberto DEPOIS que essa função retorna (o handler
// HTTP ainda vai ler dele) — o helper só pode ser removido depois que o
// stream inteiro já foi consumido, então quem chama recebe também uma
// função de limpeza pra rodar no fim (defer no handler).
func (s *Service) DownloadVolumePath(ctx context.Context, volumeName, srcPath string) (io.ReadCloser, types.ContainerPathStat, func(), error) {
	if err := validateVolumeName(volumeName); err != nil {
		return nil, types.ContainerPathStat{}, nil, err
	}
	full, err := volumeInternalPath(srcPath)
	if err != nil {
		return nil, types.ContainerPathStat{}, nil, err
	}
	if err := s.docker.PullImageIfMissing(ctx, fileHelperImage); err != nil {
		return nil, types.ContainerPathStat{}, nil, err
	}
	helperID, err := s.docker.CreateGenericContainer(ctx, docker.CreateGenericContainerInput{
		Image:   fileHelperImage,
		Command: []string{"sleep", "300"},
		Binds:   []string{volumeName + ":" + volumeMountPoint},
	})
	if err != nil {
		return nil, types.ContainerPathStat{}, nil, fmt.Errorf("preparando acesso ao volume %s: %w", volumeName, err)
	}
	cleanup := func() {
		_ = s.docker.RemoveContainer(context.Background(), helperID, "", false)
	}

	reader, stat, err := s.docker.DownloadFromContainer(ctx, helperID, full)
	if err != nil {
		cleanup()
		return nil, types.ContainerPathStat{}, nil, err
	}
	return reader, stat, cleanup, nil
}

func (s *Service) StatVolumePath(ctx context.Context, volumeName, targetPath string) (docker.FileEntry, error) {
	full, err := volumeInternalPath(targetPath)
	if err != nil {
		return docker.FileEntry{}, err
	}
	var entry docker.FileEntry
	err = s.withVolumeHelper(ctx, volumeName, func(helperID string) error {
		stat, innerErr := s.docker.StatPathInContainer(ctx, helperID, full)
		if innerErr != nil {
			return innerErr
		}
		entry = docker.FileEntry{
			Name:    stat.Name,
			Path:    targetPath,
			IsDir:   stat.Mode.IsDir(),
			Size:    stat.Size,
			Mode:    int64(stat.Mode.Perm()),
			ModTime: stat.Mtime,
		}
		return nil
	})
	return entry, err
}

func (s *Service) DeleteVolumePath(ctx context.Context, volumeName, targetPath string) error {
	full, err := volumeInternalPath(targetPath)
	if err != nil {
		return err
	}
	if full == volumeMountPoint {
		return fmt.Errorf("não é permitido operar na raiz do volume")
	}
	return s.withVolumeHelper(ctx, volumeName, func(helperID string) error {
		return s.docker.DeleteInContainer(ctx, helperID, full)
	})
}
