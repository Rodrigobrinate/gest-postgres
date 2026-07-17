package server

import (
	"context"
	"fmt"
)

// GucParam descreve um parâmetro do postgresql.conf que a plataforma expõe pra
// edição manual (modo avançado). De propósito NÃO inclui parâmetros que
// quebrariam a própria plataforma se mudados por aqui: listen_addresses/port/
// unix_socket_directories (a plataforma depende de portas/rede fixas),
// ssl_cert_file/ssl_key_file (caminho de arquivo, sem upload de certificado
// ainda), shared_preload_libraries (edição livre de texto aqui é perigosa —
// erro de sintaxe ou lib inexistente derruba o Postgres no restart; quem
// precisa disso usa o fluxo guiado da aba Desempenho), recovery_target_*/
// restore_command (só fazem sentido num cenário de restore que a plataforma
// ainda não orquestra).
type GucParam struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Label    string `json:"label"`
	Hint     string `json:"hint"`
	Restart  bool   `json:"restart"` // true = precisa reiniciar o container pra valer
}

var manageableParams = []GucParam{
	// Conexão
	{"max_connections", "Conexão", "Conexões máximas", "Quantas conexões simultâneas o servidor aceita.", true},
	{"superuser_reserved_connections", "Conexão", "Conexões reservadas p/ superuser", "Ficam de fora do limite acima, só pra admin conseguir entrar mesmo lotado.", true},
	{"authentication_timeout", "Conexão", "Timeout de autenticação", "Tempo máximo pra completar o login.", false},
	{"tcp_keepalives_idle", "Conexão", "TCP keepalive (idle)", "Segundos parado antes de mandar o primeiro keepalive.", false},
	{"tcp_keepalives_interval", "Conexão", "TCP keepalive (intervalo)", "Intervalo entre keepalives.", false},
	{"tcp_keepalives_count", "Conexão", "TCP keepalive (tentativas)", "Quantos keepalives sem resposta até considerar a conexão morta.", false},

	// Memória
	{"shared_buffers", "Memória", "Shared buffers", "Cache principal de páginas — regra de bolso: ~25% da RAM.", true},
	{"work_mem", "Memória", "Work mem", "Memória por operação de ordenação/hash. Multiplica por conexões concorrentes — cuidado.", false},
	{"maintenance_work_mem", "Memória", "Maintenance work mem", "Memória usada por VACUUM, CREATE INDEX, ALTER TABLE.", false},
	{"autovacuum_work_mem", "Memória", "Autovacuum work mem", "Memória do autovacuum — usa maintenance_work_mem se -1.", false},
	{"effective_cache_size", "Memória", "Effective cache size", "Estimativa de cache do SO pro planner — não aloca nada de verdade.", false},
	{"temp_buffers", "Memória", "Temp buffers", "Buffers pra tabelas temporárias, por sessão.", false},
	{"wal_buffers", "Memória", "WAL buffers", "Buffer de WAL antes de ir pro disco (-1 = automático).", true},
	{"max_stack_depth", "Memória", "Profundidade máx. de pilha", "Limite de stack por sessão.", false},
	{"huge_pages", "Memória", "Huge pages", "Usa hugepages do SO (try/on/off) — só liga on se o host suportar.", true},

	// WAL
	{"wal_level", "WAL", "WAL level", "Quanta informação vai no WAL (minimal/replica/logical).", true},
	{"fsync", "WAL", "fsync", "Força escrita física em disco. PERIGO: nunca desliga em produção.", false},
	{"synchronous_commit", "WAL", "Synchronous commit", "Espera confirmação de disco/réplica no COMMIT.", false},
	{"wal_writer_delay", "WAL", "WAL writer delay", "Intervalo do processo wal writer.", false},
	{"wal_compression", "WAL", "WAL compression", "Comprime imagens de página no WAL.", false},
	{"min_wal_size", "WAL", "Min WAL size", "Piso de tamanho dos segmentos de WAL reciclados.", false},
	{"max_wal_size", "WAL", "Max WAL size", "Teto — passar disso força checkpoint.", false},
	{"wal_keep_size", "WAL", "WAL keep size", "Quanto WAL manter disponível pra standbys.", false},
	{"archive_mode", "WAL", "Archive mode", "Habilita arquivamento de WAL (pré-requisito pra PITR).", true},
	{"archive_command", "WAL", "Archive command", "Comando executado pra arquivar cada segmento de WAL.", false},
	{"archive_timeout", "WAL", "Archive timeout", "Força troca de segmento de WAL a cada X tempo.", false},

	// Checkpoints
	{"checkpoint_timeout", "Checkpoints", "Checkpoint timeout", "Intervalo máximo entre checkpoints automáticos.", false},
	{"checkpoint_completion_target", "Checkpoints", "Checkpoint completion target", "Espalha a escrita do checkpoint ao longo do intervalo (0–1).", false},
	{"checkpoint_warning", "Checkpoints", "Checkpoint warning", "Avisa no log se os checkpoints tão vindo rápido demais.", false},
	{"checkpoint_flush_after", "Checkpoints", "Checkpoint flush after", "Flush incremental de páginas durante o checkpoint.", false},

	// Planner
	{"random_page_cost", "Planner", "Random page cost", "Custo estimado de leitura aleatória — baixa pra ~1.1 em SSD.", false},
	{"seq_page_cost", "Planner", "Seq page cost", "Custo estimado de leitura sequencial.", false},
	{"cpu_tuple_cost", "Planner", "CPU tuple cost", "Custo de CPU por tupla processada.", false},
	{"cpu_index_tuple_cost", "Planner", "CPU index tuple cost", "Custo de CPU por tupla de índice processada.", false},
	{"cpu_operator_cost", "Planner", "CPU operator cost", "Custo de CPU por operador/função avaliada.", false},
	{"effective_io_concurrency", "Planner", "Effective IO concurrency", "Nº de I/Os simultâneos esperados — sobe em SSD/RAID.", false},
	{"default_statistics_target", "Planner", "Default statistics target", "Granularidade das estatísticas coletadas pelo ANALYZE.", false},
	{"jit", "Planner", "JIT", "Compilação just-in-time de expressões em queries grandes.", false},
	{"from_collapse_limit", "Planner", "From collapse limit", "Limite de reescrita de subqueries no FROM.", false},
	{"join_collapse_limit", "Planner", "Join collapse limit", "Limite de reescrita de JOINs explícitos.", false},
	{"geqo_threshold", "Planner", "GEQO threshold", "A partir de quantas tabelas no join usa o otimizador genético.", false},

	// Autovacuum
	{"autovacuum", "Autovacuum", "Autovacuum ligado", "Desligar só em casos muito específicos — recomendado deixar ligado.", false},
	{"autovacuum_max_workers", "Autovacuum", "Autovacuum max workers", "Nº de workers de autovacuum rodando ao mesmo tempo.", true},
	{"autovacuum_naptime", "Autovacuum", "Autovacuum naptime", "Intervalo entre ciclos do autovacuum.", false},
	{"autovacuum_vacuum_threshold", "Autovacuum", "Vacuum threshold", "Nº fixo de linhas mortas que dispara VACUUM.", false},
	{"autovacuum_vacuum_scale_factor", "Autovacuum", "Vacuum scale factor", "Fração da tabela em linhas mortas que dispara VACUUM.", false},
	{"autovacuum_analyze_threshold", "Autovacuum", "Analyze threshold", "Nº fixo de linhas mudadas que dispara ANALYZE.", false},
	{"autovacuum_analyze_scale_factor", "Autovacuum", "Analyze scale factor", "Fração da tabela mudada que dispara ANALYZE.", false},
	{"autovacuum_vacuum_cost_delay", "Autovacuum", "Vacuum cost delay", "Pausa entre lotes de trabalho do autovacuum (throttle de I/O).", false},
	{"autovacuum_vacuum_cost_limit", "Autovacuum", "Vacuum cost limit", "Orçamento de custo antes de pausar (throttle de I/O).", false},
	{"autovacuum_freeze_max_age", "Autovacuum", "Freeze max age", "Idade máxima de transação antes de forçar vacuum — previne wraparound.", false},
	{"autovacuum_vacuum_insert_scale_factor", "Autovacuum", "Vacuum insert scale factor", "Dispara vacuum só por inserções, mesmo sem update/delete.", false},

	// Vacuum manual
	{"vacuum_cost_delay", "Vacuum", "Vacuum cost delay (manual)", "Throttle de I/O quando roda VACUUM manualmente.", false},
	{"vacuum_cost_limit", "Vacuum", "Vacuum cost limit (manual)", "Orçamento de custo do VACUUM manual.", false},
	{"vacuum_freeze_min_age", "Vacuum", "Freeze min age", "Idade mínima pra congelar uma tupla.", false},
	{"vacuum_freeze_table_age", "Vacuum", "Freeze table age", "Idade que força vacuum na tabela inteira.", false},
	{"vacuum_failsafe_age", "Vacuum", "Failsafe age", "Limite de emergência anti-wraparound.", false},

	// Replicação
	{"max_wal_senders", "Replicação", "Max WAL senders", "Nº máximo de conexões de streaming replication.", true},
	{"max_replication_slots", "Replicação", "Max replication slots", "Nº máximo de slots de replicação.", true},
	{"hot_standby", "Replicação", "Hot standby", "Permite leitura numa réplica em recuperação.", true},
	{"hot_standby_feedback", "Replicação", "Hot standby feedback", "Standby avisa o primary pra evitar conflito de vacuum.", false},
	{"synchronous_standby_names", "Replicação", "Synchronous standby names", "Define quais réplicas são síncronas.", false},

	// Logging
	{"log_min_messages", "Logging", "Log min messages", "Nível mínimo logado (DEBUG…PANIC).", false},
	{"log_min_duration_statement", "Logging", "Log de queries lentas", "Loga queries que passarem desse tempo, em ms. -1 desliga.", false},
	{"log_statement", "Logging", "Log statement", "Quais comandos logar (none/ddl/mod/all).", false},
	{"log_connections", "Logging", "Log connections", "Loga toda conexão nova.", false},
	{"log_disconnections", "Logging", "Log disconnections", "Loga toda desconexão.", false},
	{"log_lock_waits", "Logging", "Log lock waits", "Loga quando uma query espera muito por um lock.", false},
	{"log_temp_files", "Logging", "Log temp files", "Loga uso de arquivo temporário grande.", false},
	{"log_autovacuum_min_duration", "Logging", "Log autovacuum lento", "Loga execuções de autovacuum que passarem desse tempo.", false},
	{"log_line_prefix", "Logging", "Log line prefix", "Formato do prefixo de cada linha de log.", false},

	// Estatísticas
	{"track_activities", "Estatísticas", "Track activities", "Habilita o pg_stat_activity.", false},
	{"track_counts", "Estatísticas", "Track counts", "Coleta estatísticas usadas pelo autovacuum.", false},
	{"track_io_timing", "Estatísticas", "Track IO timing", "Mede tempo gasto em I/O — ajuda a achar gargalo de disco.", false},
	{"track_functions", "Estatísticas", "Track functions", "Rastreia estatísticas de chamadas de função.", false},
	{"compute_query_id", "Estatísticas", "Compute query id", "Gera ID de query, usado pra correlacionar com pg_stat_statements.", false},

	// Locks e timeouts
	{"deadlock_timeout", "Locks", "Deadlock timeout", "Tempo esperando lock antes de checar deadlock.", false},
	{"lock_timeout", "Locks", "Lock timeout", "Tempo máximo esperando um lock antes de desistir.", false},
	{"statement_timeout", "Locks", "Statement timeout", "Tempo máximo de execução de uma query. 0 = sem limite.", false},
	{"idle_in_transaction_session_timeout", "Locks", "Idle in transaction timeout", "Mata sessão parada com transação aberta depois desse tempo.", false},
	{"max_locks_per_transaction", "Locks", "Max locks per transaction", "Tamanho da tabela de locks por transação.", true},
	{"max_pred_locks_per_transaction", "Locks", "Max pred locks per transaction", "Locks de predicado, usado em SERIALIZABLE.", true},

	// Paralelismo
	{"max_worker_processes", "Paralelismo", "Max worker processes", "Total de processos em background disponíveis (paralelismo, extensões, etc).", true},
	{"max_parallel_workers", "Paralelismo", "Max parallel workers", "Pool total de workers paralelos pro cluster inteiro.", false},
	{"max_parallel_workers_per_gather", "Paralelismo", "Max parallel workers per gather", "Workers paralelos por nó de query.", false},
	{"max_parallel_maintenance_workers", "Paralelismo", "Max parallel maintenance workers", "Paralelismo em CREATE INDEX / VACUUM.", false},
}

var manageableParamSet = func() map[string]GucParam {
	m := make(map[string]GucParam, len(manageableParams))
	for _, p := range manageableParams {
		m[p.Name] = p
	}
	return m
}()

type LiveParam struct {
	GucParam
	Value          string `json:"value"`
	PendingRestart bool   `json:"pending_restart"`
}

// GetExpandedConfig lê o valor atual (já formatado pelo próprio Postgres, com
// unidade e tudo — current_setting faz isso melhor que a gente reinventando
// conversão de kB/8kB/etc na mão) de cada parâmetro gerenciável.
func (s *Service) GetExpandedConfig(ctx context.Context, id, database string) ([]LiveParam, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	names := make([]string, len(manageableParams))
	for i, p := range manageableParams {
		names[i] = p.Name
	}

	rows, err := conn.Query(ctx, `
		SELECT name, current_setting(name), pending_restart
		FROM pg_settings
		WHERE name = ANY($1)
	`, names)
	if err != nil {
		return nil, fmt.Errorf("lendo configuração: %w", err)
	}
	defer rows.Close()

	values := make(map[string]LiveParam, len(manageableParams))
	for rows.Next() {
		var name, value string
		var pending bool
		if err := rows.Scan(&name, &value, &pending); err != nil {
			return nil, fmt.Errorf("lendo parâmetro: %w", err)
		}
		values[name] = LiveParam{GucParam: manageableParamSet[name], Value: value, PendingRestart: pending}
	}

	// Preserva a ordem declarada em manageableParams (agrupada por categoria),
	// não a ordem que veio do banco.
	out := make([]LiveParam, 0, len(manageableParams))
	for _, p := range manageableParams {
		if lp, ok := values[p.Name]; ok {
			out = append(out, lp)
		}
	}
	return out, rows.Err()
}

// ApplyExpandedConfig aplica só os parâmetros que vieram em `updates` (não
// precisa mandar todo mundo de novo a cada save). Cada nome é validado contra
// a whitelist antes de virar SQL — nunca interpola nome de parâmetro vindo do
// usuário direto na query.
func (s *Service) ApplyExpandedConfig(ctx context.Context, id, database string, updates map[string]string) (bool, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return false, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	restartRequired := false
	for name, value := range updates {
		param, ok := manageableParamSet[name]
		if !ok {
			return false, fmt.Errorf("%w: parâmetro %q não é gerenciável por aqui", ErrValidation, name)
		}

		sql := fmt.Sprintf("ALTER SYSTEM SET %s = %s", name, sqlQuoteLiteral(value))
		if _, err := conn.Exec(ctx, sql); err != nil {
			return false, fmt.Errorf("%w: aplicando %s: %v", ErrValidation, name, err)
		}
		if param.Restart {
			restartRequired = true
		}
	}

	if _, err := conn.Exec(ctx, "SELECT pg_reload_conf()"); err != nil {
		return false, fmt.Errorf("recarregando config: %w", err)
	}
	return restartRequired, nil
}
