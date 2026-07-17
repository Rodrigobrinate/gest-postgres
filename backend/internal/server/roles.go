package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type RoleInfo struct {
	Name            string `json:"name"`
	CanLogin        bool   `json:"can_login"`
	Superuser       bool   `json:"superuser"`
	CreateDB        bool   `json:"create_db"`
	CreateRole      bool   `json:"create_role"`
	ConnectionLimit int    `json:"connection_limit"`
}

// ListRoles é cluster-wide (roles não pertencem a um database específico) —
// conecta em qualquer banco do servidor, tanto faz qual. Esconde os papéis
// internos pg_* (pg_signal_backend, pg_read_all_data, etc), que não são
// "usuários" de verdade.
func (s *Service) ListRoles(ctx context.Context, id string) ([]RoleInfo, error) {
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
		SELECT rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolconnlimit
		FROM pg_roles
		WHERE rolname NOT LIKE 'pg\_%'
		ORDER BY rolname
	`)
	if err != nil {
		return nil, fmt.Errorf("listando roles: %w", err)
	}
	defer rows.Close()

	roles := make([]RoleInfo, 0)
	for rows.Next() {
		var r RoleInfo
		if err := rows.Scan(&r.Name, &r.CanLogin, &r.Superuser, &r.CreateDB, &r.CreateRole, &r.ConnectionLimit); err != nil {
			return nil, fmt.Errorf("lendo role: %w", err)
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

type CreateRoleInput struct {
	Name            string `json:"name"`
	Password        string `json:"password"`
	CanLogin        bool   `json:"can_login"`
	Superuser       bool   `json:"superuser"`
	CreateDB        bool   `json:"create_db"`
	CreateRole      bool   `json:"create_role"`
	ConnectionLimit int    `json:"connection_limit"`
}

func (s *Service) CreateRole(ctx context.Context, id string, in CreateRoleInput) error {
	if !identRegex.MatchString(in.Name) {
		return fmt.Errorf("%w: nome de usuário %q inválido", ErrValidation, in.Name)
	}
	if in.ConnectionLimit == 0 {
		in.ConnectionLimit = -1 // -1 = sem limite, o default do Postgres
	}

	opts := boolOpt(in.CanLogin, "LOGIN", "NOLOGIN") + " " +
		boolOpt(in.Superuser, "SUPERUSER", "NOSUPERUSER") + " " +
		boolOpt(in.CreateDB, "CREATEDB", "NOCREATEDB") + " " +
		boolOpt(in.CreateRole, "CREATEROLE", "NOCREATEROLE")

	sql := fmt.Sprintf("CREATE ROLE %s WITH %s CONNECTION LIMIT %d",
		pgx.Identifier{in.Name}.Sanitize(), opts, in.ConnectionLimit)
	if in.Password != "" {
		sql += " PASSWORD " + sqlQuoteLiteral(in.Password)
	}

	return s.execDDL(ctx, id, "", sql)
}

func boolOpt(v bool, yes, no string) string {
	if v {
		return yes
	}
	return no
}

func (s *Service) DropRole(ctx context.Context, id, name string) error {
	if !identRegex.MatchString(name) {
		return fmt.Errorf("%w: nome de usuário %q inválido", ErrValidation, name)
	}
	return s.execDDL(ctx, id, "", "DROP ROLE "+pgx.Identifier{name}.Sanitize())
}

type TablePrivileges struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Select bool   `json:"select"`
	Insert bool   `json:"insert"`
	Update bool   `json:"update"`
	Delete bool   `json:"delete"`
}

// ListRolePrivileges cruza todas as tabelas do database com o que essa role
// especificamente tem de GRANT — tabela sem nenhum grant pra ela aparece com
// tudo false, não some da lista (senão não dá pra conceder o primeiro grant).
func (s *Service) ListRolePrivileges(ctx context.Context, id, database, role string) ([]TablePrivileges, error) {
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
			t.schemaname,
			t.tablename,
			COALESCE(bool_or(g.privilege_type = 'SELECT'), false),
			COALESCE(bool_or(g.privilege_type = 'INSERT'), false),
			COALESCE(bool_or(g.privilege_type = 'UPDATE'), false),
			COALESCE(bool_or(g.privilege_type = 'DELETE'), false)
		FROM pg_tables t
		LEFT JOIN information_schema.role_table_grants g
			ON g.table_schema = t.schemaname AND g.table_name = t.tablename AND g.grantee = $1
		WHERE t.schemaname NOT IN ('pg_catalog', 'information_schema')
		GROUP BY t.schemaname, t.tablename
		ORDER BY t.schemaname, t.tablename
	`, role)
	if err != nil {
		return nil, fmt.Errorf("lendo privilégios: %w", err)
	}
	defer rows.Close()

	privs := make([]TablePrivileges, 0)
	for rows.Next() {
		var p TablePrivileges
		if err := rows.Scan(&p.Schema, &p.Table, &p.Select, &p.Insert, &p.Update, &p.Delete); err != nil {
			return nil, fmt.Errorf("lendo linha de privilégio: %w", err)
		}
		privs = append(privs, p)
	}
	return privs, rows.Err()
}

var allowedPrivilege = map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true}

func (s *Service) SetTablePrivilege(ctx context.Context, id, database, schema, table, role, privilege string, grant bool) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(table) || !identRegex.MatchString(role) {
		return fmt.Errorf("%w: identificador inválido", ErrValidation)
	}
	if !allowedPrivilege[privilege] {
		return fmt.Errorf("%w: privilégio %q inválido", ErrValidation, privilege)
	}

	tableIdent := pgx.Identifier{schema, table}.Sanitize()
	roleIdent := pgx.Identifier{role}.Sanitize()

	var sql string
	if grant {
		sql = fmt.Sprintf("GRANT %s ON %s TO %s", privilege, tableIdent, roleIdent)
	} else {
		sql = fmt.Sprintf("REVOKE %s ON %s FROM %s", privilege, tableIdent, roleIdent)
	}
	return s.execDDL(ctx, id, database, sql)
}
