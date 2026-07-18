// Package server contém o domínio "servidor Postgres gerenciado": modelo, presets
// de recursos e a lógica de orquestração (cria container, persiste metadado, etc).
package server

import "time"

type Status string

const (
	StatusCreating  Status = "creating"
	StatusRunning   Status = "running"
	StatusStopped   Status = "stopped"
	StatusRestarting Status = "restarting"
	StatusError     Status = "error"
	StatusRemoving  Status = "removing"
)

type Preset string

const (
	PresetSmall      Preset = "small"
	PresetMedium     Preset = "medium"
	PresetLarge      Preset = "large"
	PresetCustom     Preset = "custom"
)

// Resources é o que efetivamente vira limites do container Docker + parâmetros
// de memória do postgresql.conf calculados a partir deles.
type Resources struct {
	CPUCores   float64 `json:"cpu_cores"`
	MemoryMB   int     `json:"memory_mb"`
	DiskGB     int     `json:"disk_gb"`
}

// PostgresConfig é o subset de postgresql.conf coberto pelo MVP (ver CLAUDE.md).
// Calculado automaticamente a partir de Resources quando Preset != custom, mas
// sempre editável pelo usuário depois.
type PostgresConfig struct {
	MaxConnections           int    `json:"max_connections"`
	SharedBuffersMB          int    `json:"shared_buffers_mb"`
	WorkMemMB                int    `json:"work_mem_mb"`
	MaintenanceWorkMemMB     int    `json:"maintenance_work_mem_mb"`
	EffectiveCacheSizeMB     int    `json:"effective_cache_size_mb"`
	LogMinDurationStatementMs int   `json:"log_min_duration_statement_ms"`
}

type Server struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"` // tag da imagem postgres, ex "16"
	Status      Status         `json:"status"`
	Preset      Preset         `json:"preset"`
	Resources   Resources      `json:"resources"`
	Config      PostgresConfig `json:"config"`

	HostPort int    `json:"host_port"`
	Username string `json:"username"`
	// PasswordEncrypted nunca é serializado pra fora da camada de persistência.
	PasswordEncrypted string `json:"-"`
	DatabaseName      string `json:"database_name"`

	ContainerID   string `json:"-"`
	ContainerName string `json:"container_name"`
	VolumeName    string `json:"volume_name"`

	PoolerEnabled       bool   `json:"pooler_enabled"`
	PoolerContainerID   string `json:"-"`
	PoolerContainerName string `json:"pooler_container_name,omitempty"`
	PoolerHostPort      int    `json:"pooler_host_port,omitempty"`
	PoolerPoolMode      string `json:"pooler_pool_mode"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateInput é o payload de criação vindo da API. Password vazio = gera senha forte.
type CreateInput struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Version      string    `json:"version"`
	Preset       Preset    `json:"preset"`
	Resources    Resources `json:"resources"` // usado quando Preset == custom
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	DatabaseName string    `json:"database_name"`
}
