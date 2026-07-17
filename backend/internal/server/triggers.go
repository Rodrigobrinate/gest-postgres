package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type TriggerInfo struct {
	Name         string `json:"name"`
	Schema       string `json:"schema"`
	Table        string `json:"table"`
	FunctionName string `json:"function_name"`
	Enabled      bool   `json:"enabled"`
	Definition   string `json:"definition"`
}

func (s *Service) ListTriggers(ctx context.Context, id, database, schema, table string) ([]TriggerInfo, error) {
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
			t.tgname,
			n.nspname,
			c.relname,
			p.proname,
			t.tgenabled != 'D',
			pg_get_triggerdef(t.oid)
		FROM pg_trigger t
		JOIN pg_class c ON c.oid = t.tgrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_proc p ON p.oid = t.tgfoid
		WHERE NOT t.tgisinternal
			AND n.nspname = $1 AND c.relname = $2
		ORDER BY t.tgname
	`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("listando triggers: %w", err)
	}
	defer rows.Close()

	triggers := make([]TriggerInfo, 0)
	for rows.Next() {
		var tr TriggerInfo
		if err := rows.Scan(&tr.Name, &tr.Schema, &tr.Table, &tr.FunctionName, &tr.Enabled, &tr.Definition); err != nil {
			return nil, fmt.Errorf("lendo trigger: %w", err)
		}
		triggers = append(triggers, tr)
	}
	return triggers, rows.Err()
}

// ListTriggerFunctions lista funções com retorno `trigger` — as únicas que
// servem de corpo pra CREATE TRIGGER. Alimenta o dropdown do formulário; o
// MVP não tem editor de função ainda, então o usuário precisa ter criado a
// função antes (via editor SQL).
func (s *Service) ListTriggerFunctions(ctx context.Context, id, database string) ([]string, error) {
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
		SELECT n.nspname || '.' || p.proname
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE p.prorettype = 'trigger'::regtype
			AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY 1
	`)
	if err != nil {
		return nil, fmt.Errorf("listando funções de trigger: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("lendo função: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

type CreateTriggerInput struct {
	Name         string   `json:"name"`
	Schema       string   `json:"schema"`
	Table        string   `json:"table"`
	Timing       string   `json:"timing"` // BEFORE | AFTER | INSTEAD OF
	Events       []string `json:"events"` // INSERT | UPDATE | DELETE | TRUNCATE
	Level        string   `json:"level"`  // ROW | STATEMENT
	FunctionName string   `json:"function_name"`
}

var allowedTiming = map[string]bool{"BEFORE": true, "AFTER": true, "INSTEAD OF": true}
var allowedEvent = map[string]bool{"INSERT": true, "UPDATE": true, "DELETE": true, "TRUNCATE": true}
var allowedLevel = map[string]bool{"ROW": true, "STATEMENT": true}

func (s *Service) CreateTrigger(ctx context.Context, id, database string, in CreateTriggerInput) error {
	if !identRegex.MatchString(in.Name) {
		return fmt.Errorf("%w: nome de trigger %q inválido", ErrValidation, in.Name)
	}
	if !identRegex.MatchString(in.Schema) || !identRegex.MatchString(in.Table) {
		return fmt.Errorf("%w: schema/tabela inválidos", ErrValidation)
	}
	if !allowedTiming[in.Timing] {
		return fmt.Errorf("%w: timing %q inválido", ErrValidation, in.Timing)
	}
	if !allowedLevel[in.Level] {
		return fmt.Errorf("%w: level %q inválido", ErrValidation, in.Level)
	}
	if len(in.Events) == 0 {
		return fmt.Errorf("%w: escolhe pelo menos um evento", ErrValidation)
	}
	for _, ev := range in.Events {
		if !allowedEvent[ev] {
			return fmt.Errorf("%w: evento %q inválido", ErrValidation, ev)
		}
	}

	// function_name pode vir "schema.func" ou só "func" — cada parte validada
	// e requotada separadamente, não interpolada crua.
	funcParts := strings.SplitN(in.FunctionName, ".", 2)
	for _, p := range funcParts {
		if !identRegex.MatchString(p) {
			return fmt.Errorf("%w: nome de função %q inválido", ErrValidation, in.FunctionName)
		}
	}
	funcIdent := pgx.Identifier(funcParts).Sanitize()

	sql := fmt.Sprintf(
		"CREATE TRIGGER %s %s %s ON %s FOR EACH %s EXECUTE FUNCTION %s()",
		pgx.Identifier{in.Name}.Sanitize(),
		in.Timing,
		strings.Join(in.Events, " OR "),
		pgx.Identifier{in.Schema, in.Table}.Sanitize(),
		in.Level,
		funcIdent,
	)

	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return nil
}

func (s *Service) SetTriggerEnabled(ctx context.Context, id, database, schema, table, name string, enabled bool) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(table) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: identificador inválido", ErrValidation)
	}
	action := "ENABLE"
	if !enabled {
		action = "DISABLE"
	}
	sql := fmt.Sprintf("ALTER TABLE %s %s TRIGGER %s",
		pgx.Identifier{schema, table}.Sanitize(), action, pgx.Identifier{name}.Sanitize())
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) DropTrigger(ctx context.Context, id, database, schema, table, name string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(table) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: identificador inválido", ErrValidation)
	}
	sql := fmt.Sprintf("DROP TRIGGER %s ON %s",
		pgx.Identifier{name}.Sanitize(), pgx.Identifier{schema, table}.Sanitize())
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) execDDL(ctx context.Context, id, database, sql string) error {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return nil
}
