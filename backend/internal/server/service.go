package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gest-postgres/backend/internal/crypto"
	"github.com/gest-postgres/backend/internal/docker"
	"github.com/jackc/pgx/v5"
)

var ErrValidation = fmt.Errorf("entrada inválida")

// allowedVersions é a lista de tags de imagem postgres suportadas no MVP.
// Manter sincronizado com o dropdown do wizard de criação no frontend.
var allowedVersions = map[string]bool{
	"13": true, "14": true, "15": true, "16": true, "17": true,
}

type Service struct {
	repo            *Repo
	docker          *docker.Client
	secretBox       *crypto.SecretBox
	history         *HistoryCollector
	platformHistory *platformHistory

	networkName    string
	portRangeStart int
	portRangeEnd   int
}

func NewService(repo *Repo, dockerClient *docker.Client, secretBox *crypto.SecretBox, networkName string, portRangeStart, portRangeEnd int) *Service {
	return &Service{
		repo:            repo,
		docker:          dockerClient,
		secretBox:       secretBox,
		history:         NewHistoryCollector(240), // ~1h a 15s/amostra
		platformHistory: newPlatformHistory(240),
		networkName:     networkName,
		portRangeStart:  portRangeStart,
		portRangeEnd:    portRangeEnd,
	}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Server, error) {
	if err := validateCreateInput(in); err != nil {
		return nil, err
	}

	resources := ResourcesForPreset(in.Preset, in.Resources)
	pgConfig := ConfigForResources(resources)

	password := in.Password
	if password == "" {
		var err error
		password, err = generatePassword()
		if err != nil {
			return nil, fmt.Errorf("gerando senha: %w", err)
		}
	}
	encryptedPassword, err := s.secretBox.Seal(password)
	if err != nil {
		return nil, fmt.Errorf("cifrando senha: %w", err)
	}

	hostPort, err := s.allocatePort(ctx)
	if err != nil {
		return nil, err
	}

	slug := slugify(in.Name)
	containerName := fmt.Sprintf("gestpg-%s-%d", slug, time.Now().UnixNano()%1_000_000)
	volumeName := containerName + "-data"

	databaseName := in.DatabaseName
	if databaseName == "" {
		databaseName = "app"
	}
	username := in.Username
	if username == "" {
		username = "postgres"
	}

	record := &Server{
		Name:              in.Name,
		Description:       in.Description,
		Version:           in.Version,
		Status:            StatusCreating,
		Preset:            in.Preset,
		Resources:         resources,
		Config:            pgConfig,
		HostPort:          hostPort,
		Username:          username,
		PasswordEncrypted: encryptedPassword,
		DatabaseName:      databaseName,
		ContainerName:     containerName,
		VolumeName:        volumeName,
	}

	if err := s.repo.Create(ctx, record); err != nil {
		return nil, err
	}

	go s.provision(context.WithoutCancel(ctx), record.ID, password)

	return record, nil
}

// provision roda em background (a criação de container + espera do Postgres subir
// leva alguns segundos) e vai deixando o status no metadata DB atualizado pra UI
// poder fazer polling de "creating" -> "running"/"error".
func (s *Service) provision(ctx context.Context, serverID, plainPassword string) {
	record, err := s.repo.Get(ctx, serverID)
	if err != nil {
		return
	}

	// gestpg-postgres:X é a imagem oficial + pgvector/pg_cron, buildada
	// localmente pelo setup.sh (ver postgres-image/Dockerfile) — nunca
	// baixada de registry, então PullImageIfMissing só confirma que já
	// existe local (o backend não tem permissão de build no docker-socket-proxy
	// de propósito, só de pull/inspect).
	image := "gestpg-postgres:" + record.Version

	if err := s.docker.EnsureNetwork(ctx, s.networkName); err != nil {
		s.markError(ctx, serverID)
		return
	}
	if err := s.docker.EnsureVolume(ctx, record.VolumeName); err != nil {
		s.markError(ctx, serverID)
		return
	}
	if err := s.docker.PullImageIfMissing(ctx, image); err != nil {
		s.markError(ctx, serverID)
		return
	}

	containerID, err := s.docker.CreateContainer(ctx, docker.CreateContainerInput{
		Name:         record.ContainerName,
		Image:        image,
		Username:     record.Username,
		Password:     plainPassword,
		DatabaseName: record.DatabaseName,
		HostPort:     record.HostPort,
		VolumeName:   record.VolumeName,
		NetworkName:  s.networkName,
		CPUCores:     record.Resources.CPUCores,
		MemoryMB:     record.Resources.MemoryMB,
		ServerID:     serverID,
	})
	if err != nil {
		s.markError(ctx, serverID)
		return
	}

	_ = s.repo.SetContainerID(ctx, serverID, containerID)

	if err := s.docker.WaitHealthy(ctx, containerID, 60*time.Second); err != nil {
		s.markError(ctx, serverID)
		return
	}

	// Container "running" só quer dizer que o processo subiu — no primeiro boot,
	// initdb ainda roda por alguns segundos depois disso. Confirma que o Postgres
	// já aceita conexão de verdade antes de marcar como pronto pro usuário.
	if err := waitPostgresReady(ctx, record.ContainerName, record.Username, plainPassword, record.DatabaseName, 60*time.Second); err != nil {
		s.markError(ctx, serverID)
		return
	}

	// Config calculada pelo preset entra via ALTER SYSTEM (não `-c` no comando
	// do container) — de propósito: `-c` na linha de comando tem prioridade
	// MAIOR que ALTER SYSTEM e nunca mais poderia ser trocado depois, nem
	// reiniciando o container (que sobe de novo com o mesmo `-c` fixo). Aplica
	// e já reinicia uma vez aqui pra shared_buffers/max_connections (que só
	// pegam valer com restart) já nascerem certos.
	if _, err := s.applySettings(ctx, record, record.DatabaseName, record.Config); err != nil {
		s.markError(ctx, serverID)
		return
	}

	// pg_stat_statements só coleta estatística de verdade se estiver em
	// shared_preload_libraries — só CREATE EXTENSION não basta (a extensão fica
	// instalada mas sem dados). Aba de desempenho de queries depende disso, então
	// já nasce habilitado por padrão em todo servidor novo.
	if err := s.enableQueryStatsPreload(ctx, record); err != nil {
		s.markError(ctx, serverID)
		return
	}

	if err := s.docker.RestartContainer(ctx, containerID); err != nil {
		s.markError(ctx, serverID)
		return
	}
	if err := s.docker.WaitHealthy(ctx, containerID, 60*time.Second); err != nil {
		s.markError(ctx, serverID)
		return
	}
	if err := waitPostgresReady(ctx, record.ContainerName, record.Username, plainPassword, record.DatabaseName, 60*time.Second); err != nil {
		s.markError(ctx, serverID)
		return
	}

	if err := s.enableQueryStatsExtension(ctx, record); err != nil {
		s.markError(ctx, serverID)
		return
	}

	_ = s.repo.UpdateStatus(ctx, serverID, StatusRunning)
}

func waitPostgresReady(ctx context.Context, containerName, username, password, database string, timeout time.Duration) error {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:5432/%s?sslmode=disable&connect_timeout=2",
		url.QueryEscape(username), url.QueryEscape(password), containerName, url.QueryEscape(database),
	)

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := pgx.Connect(ctx, connString)
		if err == nil {
			conn.Close(ctx)
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("postgres não ficou pronto a tempo: %w", lastErr)
}

func (s *Service) markError(ctx context.Context, serverID string) {
	_ = s.repo.UpdateStatus(ctx, serverID, StatusError)
}

func (s *Service) List(ctx context.Context) ([]*Server, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range list {
		s.refreshLiveStatus(ctx, record)
	}
	return list, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Server, error) {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.refreshLiveStatus(ctx, record)
	return record, nil
}

// refreshLiveStatus consulta o Docker pro estado real do container e reconcilia
// com o metadata DB caso tenham divergido (ex: alguém parou o container fora da
// plataforma). Best-effort: erro de inspect não derruba a chamada, só mantém o
// último status conhecido.
func (s *Service) refreshLiveStatus(ctx context.Context, record *Server) {
	if record.ContainerID == "" || record.Status == StatusCreating || record.Status == StatusError {
		return
	}
	info, err := s.docker.InspectContainer(ctx, record.ContainerID)
	if err != nil {
		return
	}
	live := mapDockerStatus(info.Status)
	if live != record.Status {
		record.Status = live
		_ = s.repo.UpdateStatus(ctx, record.ID, live)
	}
}

func (s *Service) Start(ctx context.Context, id string) error {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := requireContainer(record); err != nil {
		return err
	}
	if err := s.docker.StartContainer(ctx, record.ContainerID); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, StatusRunning)
}

func (s *Service) Stop(ctx context.Context, id string) error {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := requireContainer(record); err != nil {
		return err
	}
	if err := s.docker.StopContainer(ctx, record.ContainerID); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, StatusStopped)
}

func (s *Service) Restart(ctx context.Context, id string) error {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := requireContainer(record); err != nil {
		return err
	}
	if err := s.repo.UpdateStatus(ctx, id, StatusRestarting); err != nil {
		return err
	}
	if err := s.docker.RestartContainer(ctx, record.ContainerID); err != nil {
		s.markError(ctx, id)
		return err
	}
	return s.repo.UpdateStatus(ctx, id, StatusRunning)
}

// requireContainer barra ações de lifecycle (start/stop/restart) enquanto o
// container ainda não existe (status creating, provisionamento em background
// ainda não chamou SetContainerID) — sem isso, a chamada ia pro Docker com um
// container ID vazio e voltava um "page not found" cru e confuso.
func requireContainer(record *Server) error {
	if record.ContainerID == "" {
		return fmt.Errorf("%w: servidor ainda está sendo provisionado, aguarde", ErrValidation)
	}
	return nil
}

// Delete remove o container e, se keepVolume for false, apaga também o volume
// (perda de dados irreversível) — a confirmação explícita é responsabilidade do
// handler HTTP/frontend, aqui só executa.
func (s *Service) Delete(ctx context.Context, id string, keepVolume bool) error {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	_ = s.repo.UpdateStatus(ctx, id, StatusRemoving)
	if record.ContainerID != "" {
		if err := s.docker.RemoveContainer(ctx, record.ContainerID, record.VolumeName, !keepVolume); err != nil {
			s.markError(ctx, id)
			return err
		}
	}
	s.history.forget(id)
	return s.repo.Delete(ctx, id)
}

// Password decifra e devolve a senha em texto puro — hoje a plataforma não
// tem login/RBAC (item ainda em aberto no MVP), então quem acessa a API já é
// o admin; não faz sentido esconder a senha de quem já roda SQL arbitrário
// pelo editor.
func (s *Service) Password(ctx context.Context, id string) (string, error) {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return s.secretBox.Open(record.PasswordEncrypted)
}

// Logs retorna as últimas linhas de stdout+stderr do container — é onde a
// imagem postgres oficial manda o log do Postgres por padrão (log_destination
// = stderr, sem logging_collector). Não exige status running: útil também
// pra ver por que um container morreu.
func (s *Service) Logs(ctx context.Context, id string, tailLines int) (string, error) {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if record.ContainerID == "" {
		return "", nil
	}
	return s.docker.ContainerLogs(ctx, record.ContainerID, tailLines)
}

func (s *Service) Stats(ctx context.Context, id string) (docker.ContainerStatsSnapshot, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return docker.ContainerStatsSnapshot{}, err
	}
	return s.docker.ContainerStats(ctx, record.ContainerID)
}

func (s *Service) allocatePort(ctx context.Context) (int, error) {
	maxUsed, err := s.repo.MaxHostPort(ctx)
	if err != nil {
		return 0, err
	}
	next := maxUsed + 1
	if next < s.portRangeStart {
		next = s.portRangeStart
	}
	if next > s.portRangeEnd {
		return 0, fmt.Errorf("sem portas livres na faixa %d-%d", s.portRangeStart, s.portRangeEnd)
	}
	return next, nil
}


func mapDockerStatus(dockerStatus string) Status {
	switch dockerStatus {
	case "running":
		return StatusRunning
	case "restarting":
		return StatusRestarting
	case "created":
		return StatusCreating
	case "exited", "paused":
		return StatusStopped
	case "dead":
		return StatusError
	default:
		return StatusError
	}
}

func validateCreateInput(in CreateInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("%w: nome é obrigatório", ErrValidation)
	}
	if !allowedVersions[in.Version] {
		return fmt.Errorf("%w: versão %q não suportada", ErrValidation, in.Version)
	}
	switch in.Preset {
	case PresetSmall, PresetMedium, PresetLarge, PresetCustom:
	default:
		return fmt.Errorf("%w: preset %q inválido", ErrValidation, in.Preset)
	}
	if in.Preset == PresetCustom && in.Resources.MemoryMB <= 0 {
		return fmt.Errorf("%w: recursos customizados exigem memory_mb > 0", ErrValidation)
	}
	return nil
}

var slugInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)

func slugify(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = slugInvalidChars.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "server"
	}
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return slug
}

func generatePassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
