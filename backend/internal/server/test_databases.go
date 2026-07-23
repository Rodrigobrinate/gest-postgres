package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// DatabaseCreationResult é a resposta de qualquer fluxo que cria banco +
// role isolada dona dele (usado tanto por "Novo banco" quanto por "Criar
// banco de teste") — senha só existe nessa resposta, texto puro, uma vez;
// não fica guardada na plataforma (mesma regra de qualquer senha de role
// que não seja o superuser).
type DatabaseCreationResult struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateTestDatabase é o botão "Criar banco de teste" — cria banco + role
// (mesmo nome pros dois, pra não ter dois valores pra lembrar) + senha, com
// a role isolada: sem grant nenhum em qualquer outro banco do cluster
// (comportamento padrão de role recém-criada — a plataforma não concede
// nada além do que essa função concede aqui) e dona de tudo que
// existir/for criado no schema public DESSE banco daqui pra frente (ALTER
// DEFAULT PRIVILEGES cobre tabela/sequence criada depois, não só o que já
// existe no momento da criação, que é vazio de qualquer forma — banco
// novo).
//
// suffix é o que o usuário digita na UI (prefixo "test_" fixo, só o resto é
// campo livre — pedido explícito pra dar pra identificar o banco depois em
// vez de um sufixo hex aleatório sem significado nenhum). Vazio cai pra um
// sufixo aleatório, só como rede de segurança — a UI não deveria deixar
// submeter vazio.
//
// Não revoga CONNECT do PUBLIC nos outros bancos do cluster — isso
// restringiria acesso de QUALQUER role já existente na plataforma
// (mudança ampla e alheia ao pedido, faria mais sentido como configuração
// explícita do que como efeito colateral de criar um banco de teste). O
// isolamento de dado (não conseguir ler/escrever nada fora desse banco) já
// está garantido pela ausência de qualquer GRANT nas tabelas dos outros
// bancos.
func (s *Service) CreateTestDatabase(ctx context.Context, id, suffix string) (*DatabaseCreationResult, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}

	if suffix == "" {
		suffix, err = randomHexSuffix(4)
		if err != nil {
			return nil, fmt.Errorf("gerando nome: %w", err)
		}
	} else if !identRegex.MatchString(suffix) {
		return nil, fmt.Errorf("%w: nome %q inválido — use só letra, número e underscore, começando com letra ou underscore", ErrValidation, suffix)
	}
	name := "test_" + suffix

	password, err := s.provisionIsolatedDatabase(ctx, record, name)
	if err != nil {
		return nil, err
	}
	return &DatabaseCreationResult{Database: name, Username: name, Password: password}, nil
}

// provisionIsolatedDatabase cria uma role + banco com a role como OWNER,
// isolada (sem grant nenhum em qualquer outro banco do cluster — comportamento
// padrão de role recém-criada, a plataforma não concede nada além do que
// essa função concede aqui) e dona de tudo que existir/for criado no schema
// public DESSE banco daqui pra frente (ALTER DEFAULT PRIVILEGES cobre
// tabela/sequence criada depois, não só o que já existe no momento da
// criação, que é vazio de qualquer forma — banco novo). Usado tanto por
// "Criar banco de teste" quanto por "Novo banco" (mesmo mecanismo,
// generalizado por pedido explícito do usuário, 2026-07-23).
//
// Não revoga CONNECT do PUBLIC nos outros bancos do cluster — isso
// restringiria acesso de QUALQUER role já existente na plataforma (mudança
// ampla e alheia ao pedido). O isolamento de dado (não conseguir ler/escrever
// nada fora desse banco) já está garantido pela ausência de qualquer GRANT
// nas tabelas dos outros bancos.
func (s *Service) provisionIsolatedDatabase(ctx context.Context, record *Server, name string) (string, error) {
	nameIdent := pgx.Identifier{name}.Sanitize()

	password, err := generatePassword()
	if err != nil {
		return "", fmt.Errorf("gerando senha: %w", err)
	}

	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "CREATE ROLE "+nameIdent+" WITH LOGIN PASSWORD "+sqlQuoteLiteral(password)); err != nil {
		return "", fmt.Errorf("%w: criando usuário: %v", ErrValidation, err)
	}

	if _, err := conn.Exec(ctx, "CREATE DATABASE "+nameIdent+" OWNER "+nameIdent); err != nil {
		_, _ = conn.Exec(ctx, "DROP ROLE "+nameIdent)
		return "", fmt.Errorf("%w: criando banco: %v", ErrValidation, err)
	}

	// Conecta DENTRO do banco novo pra garantir acesso no schema public —
	// OWNER do banco já dá bastante controle, mas o schema public em si
	// tem dono próprio (herdado do template, não do dono do banco) desde o
	// Postgres 15, então sem isso a role dona do banco ainda não
	// conseguiria criar tabela.
	dbConn, err := s.connectTo(ctx, record, name)
	if err != nil {
		_, _ = conn.Exec(ctx, "DROP DATABASE "+nameIdent)
		_, _ = conn.Exec(ctx, "DROP ROLE "+nameIdent)
		return "", fmt.Errorf("conectando no banco novo pra conceder acesso: %w", err)
	}
	defer dbConn.Close(ctx)

	grants := []string{
		"GRANT ALL PRIVILEGES ON SCHEMA public TO " + nameIdent,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO " + nameIdent,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO " + nameIdent,
	}
	for _, sql := range grants {
		if _, err := dbConn.Exec(ctx, sql); err != nil {
			_, _ = conn.Exec(ctx, "DROP DATABASE "+nameIdent)
			_, _ = conn.Exec(ctx, "DROP ROLE "+nameIdent)
			return "", fmt.Errorf("%w: concedendo acesso no banco novo: %v", ErrValidation, err)
		}
	}

	return password, nil
}

func randomHexSuffix(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
