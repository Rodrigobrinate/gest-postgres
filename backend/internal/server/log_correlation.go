package server

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"
)

type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	// Level: LOG/ERROR/WARNING/FATAL/PANIC/NOTICE/INFO/DEBUG1-5 — extraído
	// do texto (não do log_line_prefix, que é editável e não tem posição
	// fixa). Vazio quando a linha não bate com nenhum nível reconhecido.
	Level string `json:"level"`
	Text  string `json:"text"`
	// Details são as linhas de continuação do MESMO evento (DETAIL/HINT/
	// STATEMENT/CONTEXT/QUERY do Postgres, ou spillover de texto multi-linha
	// sem nível próprio) — é o que a UI mostra só quando a linha é expandida.
	Details         []string `json:"details,omitempty"`
	CPUPercent      *float64 `json:"cpu_percent"`
	ConnectionCount *int     `json:"connection_count"`
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

// logLevelRegex acha o nível em qualquer posição da linha (não só no
// início) porque log_line_prefix é configurável pelo usuário (86 params
// geridos incluem esse) — procurar o token fixo ("LOG:", "ERROR:" etc.) é
// mais robusto que assumir uma posição.
var logLevelRegex = regexp.MustCompile(`\b(LOG|ERROR|WARNING|FATAL|PANIC|NOTICE|INFO|DEBUG[1-5]?|DETAIL|HINT|STATEMENT|CONTEXT|QUERY):\s`)

// primaryLevels são os que abrem uma linha NOVA na tabela; os demais
// (DETAIL/HINT/STATEMENT/CONTEXT/QUERY) são sempre continuação do evento
// anterior no Postgres — nunca aparecem sozinhos.
var primaryLevels = map[string]bool{
	"LOG": true, "ERROR": true, "WARNING": true, "FATAL": true, "PANIC": true,
	"NOTICE": true, "INFO": true,
	"DEBUG": true, "DEBUG1": true, "DEBUG2": true, "DEBUG3": true, "DEBUG4": true, "DEBUG5": true,
}

func detectLogLevel(text string) string {
	m := logLevelRegex.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

// parseLogLines espera o formato que o Docker usa quando Timestamps:true —
// "2026-07-17T23:14:20.123456789Z resto da linha". Linha sem timestamp
// parseável (não deveria acontecer, mas defensivo) entra com zero value.
// Linhas que não abrem um nível novo (DETAIL/HINT/STATEMENT/CONTEXT/QUERY do
// Postgres, ou puro spillover multi-linha sem nível nenhum) são anexadas
// como detalhe da última linha primária, nunca viram linha própria.
func parseLogLines(raw string) []LogLine {
	rawLines := strings.Split(raw, "\n")
	out := make([]LogLine, 0, len(rawLines))
	for _, l := range rawLines {
		if l == "" {
			continue
		}
		var ts time.Time
		text := l
		if sp := strings.IndexByte(l, ' '); sp >= 0 {
			if parsed, err := time.Parse(time.RFC3339Nano, l[:sp]); err == nil {
				ts = parsed
				text = l[sp+1:]
			}
		}

		level := detectLogLevel(text)
		if len(out) > 0 && !primaryLevels[level] {
			last := &out[len(out)-1]
			last.Details = append(last.Details, text)
			continue
		}
		out = append(out, LogLine{Timestamp: ts, Level: level, Text: text})
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
