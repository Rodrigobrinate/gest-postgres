package server

import (
	"context"
	"sort"
	"strings"
	"time"
)

type LogLine struct {
	Timestamp       time.Time `json:"timestamp"`
	Text            string    `json:"text"`
	CPUPercent      *float64  `json:"cpu_percent"`
	ConnectionCount *int      `json:"connection_count"`
}

// LogsTimeline busca logs com timestamp real do Docker (independe do
// log_line_prefix do Postgres) e anota cada linha com o CPU/conexões mais
// próximos no histórico de métricas — é isso que deixa dar pra "ver o log no
// mesmo lugar que o gráfico" em vez de precisar cruzar os dois manualmente.
func (s *Service) LogsTimeline(ctx context.Context, id string, tailLines int) ([]LogLine, error) {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.ContainerID == "" {
		return []LogLine{}, nil
	}

	raw, err := s.docker.ContainerLogsWithTimestamps(ctx, record.ContainerID, tailLines)
	if err != nil {
		return nil, err
	}

	lines := parseLogLines(raw)
	history := s.history.get(id)

	for i := range lines {
		cpu, conns, ok := nearestMetric(history, lines[i].Timestamp)
		if ok {
			lines[i].CPUPercent = &cpu
			lines[i].ConnectionCount = &conns
		}
	}
	return lines, nil
}

// parseLogLines espera o formato que o Docker usa quando Timestamps:true —
// "2026-07-17T23:14:20.123456789Z resto da linha". Linha sem timestamp
// parseável (não deveria acontecer, mas defensivo) entra com zero value.
func parseLogLines(raw string) []LogLine {
	rawLines := strings.Split(raw, "\n")
	out := make([]LogLine, 0, len(rawLines))
	for _, l := range rawLines {
		if l == "" {
			continue
		}
		sp := strings.IndexByte(l, ' ')
		if sp < 0 {
			out = append(out, LogLine{Text: l})
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, l[:sp])
		if err != nil {
			out = append(out, LogLine{Text: l})
			continue
		}
		out = append(out, LogLine{Timestamp: ts, Text: l[sp+1:]})
	}
	return out
}

// nearestMetric acha a amostra de métrica com timestamp mais próximo — o
// histórico é amostrado a cada ~15s (ver history.go), então "mais próximo"
// já é preciso o bastante pra correlação visual.
func nearestMetric(history []MetricPoint, at time.Time) (cpu float64, conns int, ok bool) {
	if len(history) == 0 || at.IsZero() {
		return 0, 0, false
	}
	idx := sort.Search(len(history), func(i int) bool {
		return !history[i].Timestamp.Before(at)
	})

	best := -1
	bestDiff := time.Duration(1<<63 - 1)
	for _, i := range []int{idx - 1, idx} {
		if i < 0 || i >= len(history) {
			continue
		}
		diff := history[i].Timestamp.Sub(at)
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			best = i
		}
	}
	if best == -1 {
		return 0, 0, false
	}
	return history[best].CPUPercent, history[best].ConnectionCount, true
}
