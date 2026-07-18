package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
)

const hbaPath = "/var/lib/postgresql/data/pg_hba.conf"

type HbaRule struct {
	Line     int    `json:"line"` // índice na lista retornada — usado só pra excluir, não é a linha real do arquivo
	Type     string `json:"type"`
	Database string `json:"database"`
	UserName string `json:"user_name"`
	Address  string `json:"address"` // vazio pra local
	Method   string `json:"method"`
	Raw      string `json:"raw"`
}

var allowedHbaType = map[string]bool{"local": true, "host": true, "hostssl": true, "hostnossl": true}
var allowedHbaMethod = map[string]bool{
	"trust": true, "reject": true, "scram-sha-256": true, "md5": true,
	"password": true, "cert": true,
}

// ListHbaRules lê o pg_hba.conf de dentro do container (não é SQL — esse
// arquivo não tem uma view de leitura completa e editável combinada; existe
// pg_hba_file_rules, mas ela já mostra o parse aplicado, não as linhas cruas
// que precisamos preservar/editar) e devolve só as linhas de regra real
// (ignora comentário e linha em branco).
func (s *Service) ListHbaRules(ctx context.Context, id string) ([]HbaRule, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	content, err := s.docker.ReadFileFromContainer(ctx, record.ContainerID, hbaPath)
	if err != nil {
		return nil, fmt.Errorf("lendo pg_hba.conf: %w", err)
	}
	return parseHbaRules(content), nil
}

func parseHbaRules(content []byte) []HbaRule {
	rules := make([]HbaRule, 0)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	idx := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		r := HbaRule{Line: idx, Type: fields[0], Raw: line}
		if fields[0] == "local" {
			r.Database, r.UserName, r.Method = fields[1], fields[2], fields[len(fields)-1]
		} else if len(fields) >= 5 {
			r.Database, r.UserName, r.Address, r.Method = fields[1], fields[2], fields[3], fields[len(fields)-1]
		} else {
			continue
		}
		rules = append(rules, r)
		idx++
	}
	return rules
}

type AddHbaRuleInput struct {
	Type     string `json:"type"`
	Database string `json:"database"`
	UserName string `json:"user_name"`
	Address  string `json:"address"`
	Method   string `json:"method"`
}

// AddHbaRule acrescenta uma regra no FINAL do arquivo — ordem importa em
// pg_hba.conf (primeira regra que casa vence), então "no final" é a posição
// mais segura por padrão: nunca esconde uma regra mais restritiva que já
// existia antes dela.
func (s *Service) AddHbaRule(ctx context.Context, id string, in AddHbaRuleInput) error {
	if !allowedHbaType[in.Type] {
		return fmt.Errorf("%w: tipo de regra inválido", ErrValidation)
	}
	if !allowedHbaMethod[in.Method] {
		return fmt.Errorf("%w: método de autenticação inválido", ErrValidation)
	}
	if in.Database == "" || in.UserName == "" {
		return fmt.Errorf("%w: database e usuário são obrigatórios", ErrValidation)
	}
	if in.Type != "local" && strings.TrimSpace(in.Address) == "" {
		return fmt.Errorf("%w: endereço/CIDR é obrigatório pra regras host*", ErrValidation)
	}
	for _, f := range []string{in.Database, in.UserName, in.Address} {
		if strings.ContainsAny(f, "\n\r#") {
			return fmt.Errorf("%w: campo contém caractere inválido", ErrValidation)
		}
	}

	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	content, err := s.docker.ReadFileFromContainer(ctx, record.ContainerID, hbaPath)
	if err != nil {
		return fmt.Errorf("lendo pg_hba.conf: %w", err)
	}

	var line string
	if in.Type == "local" {
		line = fmt.Sprintf("local\t%s\t%s\t%s", in.Database, in.UserName, in.Method)
	} else {
		line = fmt.Sprintf("%s\t%s\t%s\t%s\t%s", in.Type, in.Database, in.UserName, in.Address, in.Method)
	}

	newContent := ensureTrailingNewline(content) + line + "\n"
	if err := s.docker.WriteFileToContainer(ctx, record.ContainerID, hbaPath, []byte(newContent), 0600); err != nil {
		return fmt.Errorf("escrevendo pg_hba.conf: %w", err)
	}
	return s.reloadHba(ctx, record)
}

// DeleteHbaRule remove a linha cujo conteúdo (já sem espaço nas pontas) bate
// exatamente com `raw` — pedido pelo índice seria frágil (a lista que o
// frontend tem pode ficar defasada), o conteúdo exato da linha é uma chave
// mais confiável dado que cada regra tende a ser única no arquivo.
func (s *Service) DeleteHbaRule(ctx context.Context, id, raw string) error {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	content, err := s.docker.ReadFileFromContainer(ctx, record.ContainerID, hbaPath)
	if err != nil {
		return fmt.Errorf("lendo pg_hba.conf: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	var kept []string
	removed := false
	for scanner.Scan() {
		line := scanner.Text()
		if !removed && strings.TrimSpace(line) == strings.TrimSpace(raw) {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		return fmt.Errorf("%w: regra não encontrada (o arquivo pode ter mudado)", ErrValidation)
	}

	newContent := strings.Join(kept, "\n") + "\n"
	if err := s.docker.WriteFileToContainer(ctx, record.ContainerID, hbaPath, []byte(newContent), 0600); err != nil {
		return fmt.Errorf("escrevendo pg_hba.conf: %w", err)
	}
	return s.reloadHba(ctx, record)
}

// reloadHba só recarrega — diferente de shared_preload_libraries, pg_hba.conf
// nunca precisa de restart. Se a sintaxe ficar inválida, o Postgres LOGA o
// erro e continua rodando com as regras antigas em memória (não trava, não
// derruba conexão nenhuma) — bem menos perigoso do que parece à primeira vista.
func (s *Service) reloadHba(ctx context.Context, record *Server) error {
	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "SELECT pg_reload_conf()"); err != nil {
		return fmt.Errorf("recarregando pg_hba.conf: %w", err)
	}
	return nil
}

func ensureTrailingNewline(content []byte) string {
	s := string(content)
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
