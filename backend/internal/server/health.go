package server

import (
	"context"
	"fmt"
	"time"
)

// ---------- Bloat ----------

type TableBloat struct {
	Schema         string     `json:"schema"`
	Table          string     `json:"table"`
	LiveTuples     int64      `json:"live_tuples"`
	DeadTuples     int64      `json:"dead_tuples"`
	DeadRatio      float64    `json:"dead_ratio"`
	LastAutovacuum *time.Time `json:"last_autovacuum"`
	Suggestion     string     `json:"suggestion"`
}

// ListBloat usa pg_stat_user_tables (n_live_tup/n_dead_tup) como proxy barato
// de bloat — não é o tamanho físico real do inchaço (isso exigiria pgstattuple,
// extensão nem sempre instalada), mas é o sinal padrão que toda ferramenta de
// monitoramento usa pra decidir se o autovacuum tá atrasado.
func (s *Service) ListBloat(ctx context.Context, id, database string) ([]TableBloat, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT
			schemaname,
			relname,
			GREATEST(n_live_tup, 0),
			GREATEST(n_dead_tup, 0),
			last_autovacuum
		FROM pg_stat_user_tables
		WHERE n_live_tup + n_dead_tup > 0
		ORDER BY n_dead_tup DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("lendo bloat: %w", err)
	}
	defer rows.Close()

	out := make([]TableBloat, 0)
	for rows.Next() {
		var b TableBloat
		if err := rows.Scan(&b.Schema, &b.Table, &b.LiveTuples, &b.DeadTuples, &b.LastAutovacuum); err != nil {
			return nil, fmt.Errorf("lendo linha de bloat: %w", err)
		}
		total := b.LiveTuples + b.DeadTuples
		if total > 0 {
			b.DeadRatio = float64(b.DeadTuples) / float64(total)
		}
		switch {
		case b.DeadRatio >= 0.4:
			b.Suggestion = "bloat alto — rode VACUUM (ou VACUUM FULL se precisar recuperar espaço em disco) o quanto antes"
		case b.DeadRatio >= 0.2:
			b.Suggestion = "bloat considerável — autovacuum provavelmente tá atrasado pra essa tabela"
		case b.DeadRatio >= 0.1:
			b.Suggestion = "bloat moderado, normal em tabelas com muito UPDATE/DELETE"
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ---------- Wraparound ----------

type WraparoundInfo struct {
	Database string `json:"database"`
	Age      int64  `json:"age"`
	Status   string `json:"status"` // "ok" | "warning" | "critical"
}

// Limites seguem a prática comum de monitoramento Postgres: o hard stop do
// banco (recusa novas transações) acontece perto de 2^31 (~2.1B). Alertar bem
// antes disso dá tempo de rodar VACUUM manual.
const (
	wraparoundWarningAge  = 1_000_000_000
	wraparoundCriticalAge = 1_500_000_000
)

func (s *Service) WraparoundStatus(ctx context.Context, id string) ([]WraparoundInfo, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT datname, age(datfrozenxid)
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY age(datfrozenxid) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("lendo wraparound: %w", err)
	}
	defer rows.Close()

	out := make([]WraparoundInfo, 0)
	for rows.Next() {
		var w WraparoundInfo
		if err := rows.Scan(&w.Database, &w.Age); err != nil {
			return nil, fmt.Errorf("lendo linha de wraparound: %w", err)
		}
		switch {
		case w.Age >= wraparoundCriticalAge:
			w.Status = "critical"
		case w.Age >= wraparoundWarningAge:
			w.Status = "warning"
		default:
			w.Status = "ok"
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ---------- Health score ----------

type HealthFactor struct {
	Name   string `json:"name"`
	Score  int    `json:"score"` // 0-100
	Detail string `json:"detail"`
}

type HealthScore struct {
	Score   int            `json:"score"` // 0-100, média dos fatores
	Factors []HealthFactor `json:"factors"`
}

// GetHealthScore combina sinais baratos de já ter tudo calculado em outro
// lugar (conexões, cache hit, bloat, wraparound) num número único fácil de
// olhar no dashboard. Pesos são heurística, não ciência — o valor é o "olha
// aqui" que aponta pra aba certa, não uma métrica precisa.
func (s *Service) GetHealthScore(ctx context.Context, id, database string) (*HealthScore, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	factors := make([]HealthFactor, 0, 4)

	var used, max int
	err = conn.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM pg_stat_activity WHERE backend_type = 'client backend'),
			(SELECT setting::int FROM pg_settings WHERE name = 'max_connections')
	`).Scan(&used, &max)
	if err == nil && max > 0 {
		pct := float64(used) / float64(max)
		score := 100 - int(pct*100)
		if score < 0 {
			score = 0
		}
		factors = append(factors, HealthFactor{
			Name:   "Conexões",
			Score:  score,
			Detail: fmt.Sprintf("%d de %d (%.0f%%)", used, max, pct*100),
		})
	}

	var hit, read float64
	err = conn.QueryRow(ctx, `
		SELECT
			COALESCE(sum(blks_hit), 0),
			COALESCE(sum(blks_hit) + sum(blks_read), 0)
		FROM pg_stat_database
	`).Scan(&hit, &read)
	if err == nil && read > 0 {
		ratio := hit / read
		factors = append(factors, HealthFactor{
			Name:   "Cache hit ratio",
			Score:  int(ratio * 100),
			Detail: fmt.Sprintf("%.1f%%", ratio*100),
		})
	}

	bloat, err := s.ListBloat(ctx, id, database)
	if err == nil {
		worst := 0.0
		for _, b := range bloat {
			if b.DeadRatio > worst {
				worst = b.DeadRatio
			}
		}
		score := 100 - int(worst*100)
		if score < 0 {
			score = 0
		}
		factors = append(factors, HealthFactor{
			Name:   "Bloat",
			Score:  score,
			Detail: fmt.Sprintf("pior tabela: %.0f%% de tuplas mortas", worst*100),
		})
	}

	wrap, err := s.WraparoundStatus(ctx, id)
	if err == nil {
		worstAge := int64(0)
		for _, w := range wrap {
			if w.Age > worstAge {
				worstAge = w.Age
			}
		}
		score := 100
		switch {
		case worstAge >= wraparoundCriticalAge:
			score = 0
		case worstAge >= wraparoundWarningAge:
			score = 40
		}
		factors = append(factors, HealthFactor{
			Name:   "Wraparound",
			Score:  score,
			Detail: fmt.Sprintf("idade máxima: %d transações", worstAge),
		})
	}

	if len(factors) == 0 {
		return &HealthScore{Score: 0, Factors: factors}, nil
	}
	total := 0
	for _, f := range factors {
		total += f.Score
	}
	return &HealthScore{Score: total / len(factors), Factors: factors}, nil
}

// ---------- Previsão de capacidade ----------

type CapacityForecast struct {
	CurrentDiskMB float64  `json:"current_disk_mb"`
	DiskLimitMB   float64  `json:"disk_limit_mb"`
	TrendMBPerDay float64  `json:"trend_mb_per_day"`
	DaysUntilFull *float64 `json:"days_until_full"`
	SampleWindow  string   `json:"sample_window"`
}

// GetCapacityForecast projeta quando o disco enche a partir da tendência de
// crescimento observada na janela de histórico em memória (~1h por padrão,
// reseta se o backend reiniciar — ver history.go). Regressão linear simples;
// com pouca amostra a projeção é só uma estimativa grosseira, não uma promessa.
func (s *Service) GetCapacityForecast(ctx context.Context, id string) (*CapacityForecast, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}

	points := s.history.get(id)
	limitMB := float64(record.Resources.DiskGB) * 1024

	if len(points) == 0 {
		return &CapacityForecast{DiskLimitMB: limitMB, SampleWindow: "sem amostras ainda"}, nil
	}

	current := points[len(points)-1].DiskUsedMB
	forecast := &CapacityForecast{
		CurrentDiskMB: current,
		DiskLimitMB:   limitMB,
		SampleWindow:  fmt.Sprintf("%d amostras", len(points)),
	}

	if len(points) < 3 {
		return forecast, nil
	}

	t0 := points[0].Timestamp
	var sumX, sumY, sumXY, sumXX float64
	n := float64(len(points))
	for _, p := range points {
		x := p.Timestamp.Sub(t0).Hours()
		y := p.DiskUsedMB
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return forecast, nil
	}
	slopePerHour := (n*sumXY - sumX*sumY) / denom
	trendPerDay := slopePerHour * 24
	forecast.TrendMBPerDay = trendPerDay

	// Tendência praticamente zero (mas tecnicamente positiva, por ruído de
	// amostragem) projeta "disco cheio em 1 milhão de anos" — não é uma
	// previsão útil. Só reporta um prazo se ele cair numa janela plausível.
	const maxPlausibleDays = 3650 // 10 anos
	if trendPerDay > 0 && current < limitMB {
		days := (limitMB - current) / trendPerDay
		if days <= maxPlausibleDays {
			forecast.DaysUntilFull = &days
		}
	}
	return forecast, nil
}
