package infra

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const cronOutputMaxLen = 8000

type CronJob struct {
	ID              string     `json:"id"`
	ContainerID     string     `json:"container_id"`
	ContainerName   string     `json:"container_name"`
	Name            string     `json:"name"`
	Command         string     `json:"command"`
	Frequency       string     `json:"frequency"`
	IntervalMinutes *int       `json:"interval_minutes,omitempty"`
	Weekday         *int       `json:"weekday,omitempty"`
	TimeOfDay       string     `json:"time_of_day"`
	Enabled         bool       `json:"enabled"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	LastExitCode    *int       `json:"last_exit_code,omitempty"`
	LastOutput      string     `json:"last_output,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type CreateCronJobInput struct {
	ContainerID     string `json:"container_id"`
	ContainerName   string `json:"container_name"`
	Name            string `json:"name"`
	Command         string `json:"command"`
	Frequency       string `json:"frequency"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	Weekday         int    `json:"weekday,omitempty"`
	TimeOfDay       string `json:"time_of_day,omitempty"`
}

var timeOfDayRegex = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func (s *Service) CreateCronJob(ctx context.Context, in CreateCronJobInput) (*CronJob, error) {
	if in.ContainerID == "" || in.Name == "" || in.Command == "" {
		return nil, fmt.Errorf("container, nome e comando são obrigatórios")
	}
	var intervalMinutes *int
	var weekday *int
	timeOfDay := in.TimeOfDay
	if timeOfDay == "" {
		timeOfDay = "00:00"
	}

	switch in.Frequency {
	case "interval":
		if in.IntervalMinutes < 1 {
			return nil, fmt.Errorf("intervalo precisa ser de pelo menos 1 minuto")
		}
		intervalMinutes = &in.IntervalMinutes
	case "daily":
		if !timeOfDayRegex.MatchString(timeOfDay) {
			return nil, fmt.Errorf("horário inválido — use HH:MM")
		}
	case "weekly":
		if in.Weekday < 0 || in.Weekday > 6 {
			return nil, fmt.Errorf("dia da semana inválido")
		}
		if !timeOfDayRegex.MatchString(timeOfDay) {
			return nil, fmt.Errorf("horário inválido — use HH:MM")
		}
		weekday = &in.Weekday
	default:
		return nil, fmt.Errorf("frequência deve ser 'interval', 'daily' ou 'weekly'")
	}

	var j CronJob
	err := s.pool.QueryRow(ctx, `
		INSERT INTO cron_jobs (container_id, container_name, name, command, frequency, interval_minutes, weekday, time_of_day)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text, container_id, container_name, name, command, frequency, interval_minutes, weekday, time_of_day, enabled, created_at
	`, in.ContainerID, in.ContainerName, in.Name, in.Command, in.Frequency, intervalMinutes, weekday, timeOfDay).Scan(
		&j.ID, &j.ContainerID, &j.ContainerName, &j.Name, &j.Command, &j.Frequency,
		&j.IntervalMinutes, &j.Weekday, &j.TimeOfDay, &j.Enabled, &j.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("salvando cron job: %w", err)
	}
	return &j, nil
}

func (s *Service) ListCronJobs(ctx context.Context, containerID string) ([]CronJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, container_id, container_name, name, command, frequency, interval_minutes, weekday,
		       time_of_day, enabled, last_run_at, last_exit_code, last_output, created_at
		FROM cron_jobs WHERE container_id = $1 ORDER BY created_at
	`, containerID)
	if err != nil {
		return nil, fmt.Errorf("listando cron jobs: %w", err)
	}
	defer rows.Close()
	out := []CronJob{}
	for rows.Next() {
		var j CronJob
		if err := rows.Scan(
			&j.ID, &j.ContainerID, &j.ContainerName, &j.Name, &j.Command, &j.Frequency, &j.IntervalMinutes, &j.Weekday,
			&j.TimeOfDay, &j.Enabled, &j.LastRunAt, &j.LastExitCode, &j.LastOutput, &j.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("lendo cron job: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Service) DeleteCronJob(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM cron_jobs WHERE id = $1`, id); err != nil {
		return fmt.Errorf("removendo cron job: %w", err)
	}
	return nil
}

func (s *Service) SetCronJobEnabled(ctx context.Context, id string, enabled bool) error {
	if _, err := s.pool.Exec(ctx, `UPDATE cron_jobs SET enabled = $2, updated_at = now() WHERE id = $1`, id, enabled); err != nil {
		return fmt.Errorf("atualizando cron job: %w", err)
	}
	return nil
}

// RunCronJobNow dispara o job na hora, fora do agendamento normal — usado
// pelo botão "rodar agora" da UI.
func (s *Service) RunCronJobNow(ctx context.Context, id string) (*CronJob, error) {
	var j CronJob
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, container_id, container_name, name, command, frequency, interval_minutes, weekday, time_of_day, enabled, created_at
		FROM cron_jobs WHERE id = $1
	`, id).Scan(&j.ID, &j.ContainerID, &j.ContainerName, &j.Name, &j.Command, &j.Frequency, &j.IntervalMinutes, &j.Weekday, &j.TimeOfDay, &j.Enabled, &j.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("lendo cron job: %w", err)
	}
	s.runCronJob(ctx, j)
	return s.getCronJob(ctx, id)
}

func (s *Service) getCronJob(ctx context.Context, id string) (*CronJob, error) {
	var j CronJob
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, container_id, container_name, name, command, frequency, interval_minutes, weekday,
		       time_of_day, enabled, last_run_at, last_exit_code, last_output, created_at
		FROM cron_jobs WHERE id = $1
	`, id).Scan(
		&j.ID, &j.ContainerID, &j.ContainerName, &j.Name, &j.Command, &j.Frequency, &j.IntervalMinutes, &j.Weekday,
		&j.TimeOfDay, &j.Enabled, &j.LastRunAt, &j.LastExitCode, &j.LastOutput, &j.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("lendo cron job: %w", err)
	}
	return &j, nil
}

func (s *Service) runCronJob(ctx context.Context, j CronJob) {
	exitCode, output, err := s.docker.ExecRun(ctx, j.ContainerID, []string{"sh", "-c", j.Command})
	if err != nil {
		output = "falha ao executar: " + err.Error()
		exitCode = -1
	}
	if len(output) > cronOutputMaxLen {
		output = output[:cronOutputMaxLen] + "\n... (truncado)"
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE cron_jobs SET last_run_at = now(), last_exit_code = $2, last_output = $3, updated_at = now() WHERE id = $1
	`, j.ID, exitCode, output); err != nil {
		return
	}
}

func isDueTimeOfDay(timeOfDay string, now time.Time) bool {
	parts := strings.SplitN(timeOfDay, ":", 2)
	if len(parts) != 2 {
		return false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return now.UTC().Hour() == h && now.UTC().Minute() == m
}

func sameUTCDay(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

func isCronJobDue(j CronJob, now time.Time) bool {
	if !j.Enabled {
		return false
	}
	switch j.Frequency {
	case "interval":
		if j.IntervalMinutes == nil || *j.IntervalMinutes < 1 {
			return false
		}
		if j.LastRunAt == nil {
			return true
		}
		return now.Sub(*j.LastRunAt) >= time.Duration(*j.IntervalMinutes)*time.Minute
	case "daily":
		if !isDueTimeOfDay(j.TimeOfDay, now) {
			return false
		}
		return j.LastRunAt == nil || !sameUTCDay(*j.LastRunAt, now)
	case "weekly":
		if j.Weekday == nil || int(now.UTC().Weekday()) != *j.Weekday {
			return false
		}
		if !isDueTimeOfDay(j.TimeOfDay, now) {
			return false
		}
		return j.LastRunAt == nil || !sameUTCDay(*j.LastRunAt, now)
	}
	return false
}

// RunCronSweep roda em background (chamado uma vez no main), checando a
// cada `interval` quais jobs estão vencidos — mesmo padrão dos outros
// sweeps do projeto (backup, retention, alert). Cada job devido roda no
// próprio goroutine pra um comando lento não atrasar a checagem dos outros.
func (s *Service) RunCronSweep(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := s.pool.Query(ctx, `
				SELECT id::text, container_id, container_name, name, command, frequency, interval_minutes, weekday,
				       time_of_day, enabled, last_run_at, last_exit_code, last_output, created_at
				FROM cron_jobs WHERE enabled = true
			`)
			if err != nil {
				continue
			}
			var due []CronJob
			for rows.Next() {
				var j CronJob
				if err := rows.Scan(
					&j.ID, &j.ContainerID, &j.ContainerName, &j.Name, &j.Command, &j.Frequency, &j.IntervalMinutes, &j.Weekday,
					&j.TimeOfDay, &j.Enabled, &j.LastRunAt, &j.LastExitCode, &j.LastOutput, &j.CreatedAt,
				); err != nil {
					continue
				}
				if isCronJobDue(j, time.Now()) {
					due = append(due, j)
				}
			}
			rows.Close()

			for _, j := range due {
				go s.runCronJob(context.Background(), j)
			}
		}
	}
}
