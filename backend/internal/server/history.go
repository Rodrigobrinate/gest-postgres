package server

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type MetricPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryUsedMB    float64   `json:"memory_used_mb"`
	ConnectionCount int       `json:"connection_count"`
	DiskUsedMB      float64   `json:"disk_used_mb"`
	// DatabaseSizesMB é o mesmo dado de DiskUsedMB, só que aberto por banco
	// (nome -> MB) — usado pelo gráfico de linhas por banco na aba "Bancos
	// de dados". omitempty pra não inflar o payload de quem só quer o total.
	DatabaseSizesMB map[string]float64 `json:"database_sizes_mb,omitempty"`
	// ConnectionsByDatabase é o mesmo dado de ConnectionCount, aberto por
	// banco — mesmo raciocínio de DatabaseSizesMB, usado pelo gráfico de
	// linhas "Conexões por banco".
	ConnectionsByDatabase map[string]int `json:"connections_by_database,omitempty"`
	// ReadTuplesPerSec/WriteTuplesPerSec vêm de pg_stat_database (soma de
	// todo banco não-template) — taxa, não acumulado (pg_stat_database
	// guarda contador desde o último pg_stat_reset(), então precisa de
	// delta entre polls pra virar "atividade agora", mesma lição já
	// aplicada em IOPS por container/host). 0 até o segundo poll do
	// servidor.
	ReadTuplesPerSec  float64 `json:"read_tuples_per_sec"`
	WriteTuplesPerSec float64 `json:"write_tuples_per_sec"`
}

// HistoryCollector guarda uma janela recente de métricas por servidor, só em
// memória — reseta se o backend reiniciar. Suficiente pro MVP (gráfico "última
// hora"); virar série temporal persistida é backlog (REQUISITOS.md §7).
type HistoryCollector struct {
	mu     sync.Mutex
	points map[string][]MetricPoint
	maxLen int
}

func NewHistoryCollector(maxLen int) *HistoryCollector {
	return &HistoryCollector{points: make(map[string][]MetricPoint), maxLen: maxLen}
}

func (h *HistoryCollector) append(serverID string, p MetricPoint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	pts := append(h.points[serverID], p)
	if len(pts) > h.maxLen {
		pts = pts[len(pts)-h.maxLen:]
	}
	h.points[serverID] = pts
}

func (h *HistoryCollector) get(serverID string) []MetricPoint {
	h.mu.Lock()
	defer h.mu.Unlock()
	pts := h.points[serverID]
	out := make([]MetricPoint, len(pts))
	copy(out, pts)
	return out
}

func (h *HistoryCollector) forget(serverID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.points, serverID)
}

// GetMetricsHistory devolve o histórico de um servidor. rangeDur <= 1h usa
// só o buffer em memória (rápido, sem round-trip no banco — cobre a última
// ~1h, que é tudo que esse buffer já guardava antes de metric_history
// existir). rangeDur maior consulta metric_history (raw + hourly, ver
// metric_query.go), que sobrevive a reinício do backend.
func (s *Service) GetMetricsHistory(ctx context.Context, id string, rangeDur time.Duration) ([]MetricPoint, error) {
	if rangeDur <= metricHistoryFromDBWindow {
		return s.history.get(id), nil
	}
	return s.getServerMetricHistoryDB(ctx, id, time.Now().Add(-rangeDur))
}

// RunMetricsCollector roda em background (chamado uma vez no main) amostrando
// CPU/mem/conexões de todo servidor "running" a cada `interval`. Best-effort:
// erro em um servidor não afeta os outros nem para o loop.
func (s *Service) RunMetricsCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectMetricsOnce(ctx)
		}
	}
}

func (s *Service) collectMetricsOnce(ctx context.Context) {
	records, err := s.repo.List(ctx)
	if err != nil {
		return
	}

	now := time.Now()
	for _, record := range records {
		if record.Status != StatusRunning || record.ContainerID == "" {
			continue
		}

		stats, err := s.docker.ContainerStats(ctx, record.ContainerID)
		if err != nil {
			slog.Warn("coleta de métricas: falha lendo stats do container", "server_id", record.ID, "error", err)
			continue
		}

		connsByDB, err := s.connectionsByDatabase(ctx, record)
		connCount := -1
		if err != nil {
			slog.Warn("coleta de métricas: falha contando conexões", "server_id", record.ID, "error", err)
		} else {
			connCount = 0
			for _, n := range connsByDB {
				connCount += n
			}
		}

		dbSizes, err := s.databaseSizesMB(ctx, record)
		if err != nil {
			slog.Warn("coleta de métricas: falha lendo tamanho dos bancos", "server_id", record.ID, "error", err)
		}
		var diskMB float64
		for _, mb := range dbSizes {
			diskMB += mb
		}

		var readTuplesPerSec, writeTuplesPerSec float64
		if reads, writes, err := s.tuplesReadWrite(ctx, record); err != nil {
			slog.Warn("coleta de métricas: falha lendo leituras/escritas", "server_id", record.ID, "error", err)
		} else {
			readTuplesPerSec, writeTuplesPerSec = tuplesPerSecRate(record.ID, reads, writes)
		}

		point := MetricPoint{
			Timestamp:             now,
			CPUPercent:            stats.CPUPercent,
			MemoryUsedMB:          stats.MemoryUsedMB,
			ConnectionCount:       connCount,
			DiskUsedMB:            diskMB,
			DatabaseSizesMB:       dbSizes,
			ConnectionsByDatabase: connsByDB,
			ReadTuplesPerSec:      readTuplesPerSec,
			WriteTuplesPerSec:     writeTuplesPerSec,
		}
		s.history.append(record.ID, point)
		// Grava no banco de metadados além de memória — sobrevive a
		// reinício do backend (update, restart, deploy). Best-effort: uma
		// falha de escrita aqui não pode derrubar a coleta em memória, que
		// alimenta o dashboard ao vivo.
		s.recordServerMetricRaw(ctx, record.ID, point)
	}
}

// connectionsByDatabase devolve o número de conexões de CADA banco numa
// tacada só (GROUP BY datname) — usado tanto pro total agregado (aba
// Monitoramento, gráfico "Conexões") quanto pro breakdown por banco
// (gráfico de linhas "Conexões por banco"), mesmo raciocínio de
// databaseSizesMB. Sessão sem banco associado (ex: processo em background
// do próprio Postgres) fica de fora — backend_type='client backend' já
// filtra isso.
func (s *Service) connectionsByDatabase(ctx context.Context, record *Server) (map[string]int, error) {
	conn, err := s.connectTo(ctx, record, record.DatabaseName)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT datname, count(*) FROM pg_stat_activity
		WHERE backend_type = 'client backend' AND datname IS NOT NULL
		GROUP BY datname
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			return nil, err
		}
		counts[name] = n
	}
	return counts, rows.Err()
}

// databaseSizesMB devolve o tamanho de CADA banco (não-template) em MB numa
// tacada só — usado tanto pro total agregado (aba Monitoramento, gráfico
// "Disco") quanto pro breakdown por banco (aba Bancos de dados, gráfico de
// linhas), sem precisar de uma segunda conexão/query separada pra cada um.
func (s *Service) databaseSizesMB(ctx context.Context, record *Server) (map[string]float64, error) {
	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `SELECT datname, pg_database_size(datname) FROM pg_database WHERE datistemplate = false`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sizes := make(map[string]float64)
	for rows.Next() {
		var name string
		var bytes int64
		if err := rows.Scan(&name, &bytes); err != nil {
			return nil, err
		}
		sizes[name] = float64(bytes) / (1024 * 1024)
	}
	return sizes, rows.Err()
}

// tuplesReadWrite soma leitura/escrita de linha (tuple) de TODO banco
// não-template do servidor, via pg_stat_database — "leitura" é
// tup_returned+tup_fetched (linha devolvida por scan, sequencial ou por
// índice), "escrita" é tup_inserted+tup_updated+tup_deleted (qualquer DML).
// Contador acumulado desde o último pg_stat_reset() do cluster (não reseta
// sozinho) — quem chama isso precisa converter em taxa (ver
// tuplesPerSecRate), não usar cru.
func (s *Service) tuplesReadWrite(ctx context.Context, record *Server) (reads, writes int64, err error) {
	conn, err := s.connectTo(ctx, record, record.DatabaseName)
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close(ctx)

	err = conn.QueryRow(ctx, `
		SELECT
			COALESCE(sum(s.tup_returned + s.tup_fetched), 0),
			COALESCE(sum(s.tup_inserted + s.tup_updated + s.tup_deleted), 0)
		FROM pg_stat_database s
		JOIN pg_database d ON d.oid = s.datid
		WHERE d.datistemplate = false
	`).Scan(&reads, &writes)
	return reads, writes, err
}

// tupleRateSample/tupleRateLast guardam a última leitura acumulada POR
// SERVIDOR — mesmo raciocínio de containerIOPSRate (platform_stats.go) e
// HostIOPS (docker/hostiops.go), aplicado aqui em vez de duplicar a
// estrutura genérica só por causa do tipo da chave.
type tupleRateSample struct {
	reads, writes int64
	at            time.Time
}

var (
	tupleRateMu   sync.Mutex
	tupleRateLast = make(map[string]tupleRateSample)
)

func tuplesPerSecRate(serverID string, reads, writes int64) (readsPerSec, writesPerSec float64) {
	now := time.Now()

	tupleRateMu.Lock()
	prev, ok := tupleRateLast[serverID]
	tupleRateLast[serverID] = tupleRateSample{reads, writes, now}
	tupleRateMu.Unlock()

	if !ok {
		return 0, 0
	}
	dt := now.Sub(prev.at).Seconds()
	if dt <= 0 {
		return 0, 0
	}
	readsPerSec = float64(reads-prev.reads) / dt
	writesPerSec = float64(writes-prev.writes) / dt
	if readsPerSec < 0 {
		readsPerSec = 0
	}
	if writesPerSec < 0 {
		writesPerSec = 0
	}
	return readsPerSec, writesPerSec
}
