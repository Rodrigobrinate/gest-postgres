const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type ServerStatus =
  | "creating"
  | "running"
  | "stopped"
  | "restarting"
  | "error"
  | "removing";

export type Preset = "small" | "medium" | "large" | "custom";

export interface Resources {
  cpu_cores: number;
  memory_mb: number;
  disk_gb: number;
}

export interface PostgresConfig {
  max_connections: number;
  shared_buffers_mb: number;
  work_mem_mb: number;
  maintenance_work_mem_mb: number;
  effective_cache_size_mb: number;
  log_min_duration_statement_ms: number;
}

export interface ManagedServer {
  id: string;
  name: string;
  description: string;
  version: string;
  status: ServerStatus;
  preset: Preset;
  resources: Resources;
  config: PostgresConfig;
  host_port: number;
  username: string;
  database_name: string;
  container_name: string;
  volume_name: string;
  pooler_enabled: boolean;
  pooler_container_name?: string;
  pooler_host_port?: number;
  pooler_pool_mode: string;
  created_at: string;
  updated_at: string;
}

export interface UpdateServerInput {
  name?: string;
  resources?: Resources;
  host_port?: number;
}

export interface CreateServerInput {
  name: string;
  description?: string;
  version: string;
  preset: Preset;
  resources?: Resources;
  username?: string;
  password?: string;
  database_name?: string;
}

export interface QueryResult {
  columns: string[];
  rows: unknown[][];
  row_count: number;
  command_tag?: string;
  duration_ms: number;
}

export interface TableInfo {
  schema: string;
  name: string;
  size_bytes: number;
  estimated_rows: number;
}

export interface TableRowsResult {
  columns: string[];
  rows: unknown[][];
  total_rows: number;
  limit: number;
  offset: number;
  duration_ms: number;
}

export interface ActivitySession {
  pid: number;
  username: string;
  database: string;
  application_name: string;
  client_addr: string;
  state: string;
  query: string;
  query_start: string | null;
  backend_start: string | null;
}

export interface ContainerStats {
  cpu_percent: number;
  memory_used_mb: number;
  memory_limit_mb: number;
  memory_percent: number;
}

export interface Extension {
  name: string;
  default_version: string;
  installed_version: string;
  comment: string;
}

export interface GucParam {
  name: string;
  category: string;
  label: string;
  hint: string;
  restart: boolean;
}

export interface LiveParam extends GucParam {
  value: string;
  pending_restart: boolean;
}

export interface ColumnDef {
  name: string;
  type: string;
  not_null: boolean;
  primary_key: boolean;
  default: string;
}

export interface CreateTableInput {
  schema: string;
  name: string;
  columns: ColumnDef[];
}

export interface ViewInfo {
  schema: string;
  name: string;
  definition: string;
}

export interface MaterializedViewInfo {
  schema: string;
  name: string;
  populated: boolean;
  size_bytes: number;
  definition: string;
}

export interface SequenceInfo {
  schema: string;
  name: string;
  last_value: number | null;
  increment: number;
  min_value: number;
  max_value: number;
  cache_size: number;
  cycle: boolean;
}

export interface CreateSequenceInput {
  schema: string;
  name: string;
  increment: number;
  start_with: number;
  cycle: boolean;
}

export interface TypeInfo {
  schema: string;
  name: string;
  kind: "enum" | "domain" | "composite";
  detail: string;
}

export interface FunctionInfo {
  schema: string;
  name: string;
  arguments: string;
  return_type: string;
  kind: "function" | "procedure";
  language: string;
  definition: string;
  identity_args: string;
}

export interface SlowQuery {
  query_id: number;
  query: string;
  calls: number;
  total_exec_ms: number;
  mean_exec_ms: number;
  rows: number;
  cache_hit_ratio: number;
}

export interface RoleInfo {
  name: string;
  can_login: boolean;
  superuser: boolean;
  create_db: boolean;
  create_role: boolean;
  connection_limit: number;
}

export interface CreateRoleInput {
  name: string;
  password: string;
  can_login: boolean;
  superuser: boolean;
  create_db: boolean;
  create_role: boolean;
  connection_limit: number;
}

export interface TablePrivileges {
  schema: string;
  table: string;
  select: boolean;
  insert: boolean;
  update: boolean;
  delete: boolean;
}

export type Privilege = "select" | "insert" | "update" | "delete";

export interface DatabaseSize {
  name: string;
  size_bytes: number;
}

export interface MetricPoint {
  timestamp: string;
  cpu_percent: number;
  memory_used_mb: number;
  connection_count: number;
  disk_used_mb: number;
}

export interface TableBloat {
  schema: string;
  table: string;
  live_tuples: number;
  dead_tuples: number;
  dead_ratio: number;
  last_autovacuum: string | null;
  suggestion: string;
}

export interface WraparoundInfo {
  database: string;
  age: number;
  status: "ok" | "warning" | "critical";
}

export interface HealthFactor {
  name: string;
  score: number;
  detail: string;
}

export interface HealthScore {
  score: number;
  factors: HealthFactor[];
}

export interface PlanNode {
  "Node Type": string;
  "Relation Name"?: string;
  "Index Name"?: string;
  "Alias"?: string;
  "Startup Cost"?: number;
  "Total Cost"?: number;
  "Plan Rows"?: number;
  "Plan Width"?: number;
  "Actual Startup Time"?: number;
  "Actual Total Time"?: number;
  "Actual Rows"?: number;
  "Actual Loops"?: number;
  "Filter"?: string;
  "Index Cond"?: string;
  "Join Type"?: string;
  "Plans"?: PlanNode[];
  [key: string]: unknown;
}

export interface ExplainResult {
  plan: PlanNode;
  planning_time_ms?: number;
  execution_time_ms?: number;
}

export interface ContainerStat {
  container_id: string;
  name: string;
  image: string;
  is_managed: boolean;
  adoptable: boolean;
  server_id?: string;
  server_name?: string;
  cpu_percent: number;
  memory_used_mb: number;
  memory_limit_mb: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  block_read_bytes: number;
  block_write_bytes: number;
  block_read_ops: number;
  block_write_ops: number;
  volume_size_bytes?: number;
}

export interface PlatformStats {
  containers: ContainerStat[];
  total_cpu_percent: number;
  total_memory_used_mb: number;
  total_memory_limit_mb: number;
  disk_total_bytes: number;
  disk_used_bytes: number;
  disk_free_bytes: number;
  disk_available: boolean;
  docker_disk_used_bytes: number;
  network_rx_bytes_total: number;
  network_tx_bytes_total: number;
}

export interface PlatformMetricPoint {
  timestamp: string;
  cpu_percent: number;
  memory_used_mb: number;
  disk_used_bytes: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
}

export interface DiscoveredContainer {
  container_id: string;
  name: string;
  image: string;
  state: string;
  ports: string[];
}

export interface RegisterDiscoveredInput {
  name: string;
  username: string;
  password: string;
  database_name: string;
}

export interface LogLine {
  timestamp: string;
  text: string;
  cpu_percent: number | null;
  connection_count: number | null;
}

export type AlertMetric = "connections_pct" | "disk_pct" | "long_running_query_seconds" | "deadlocks";

export interface AlertRule {
  id: string;
  server_id: string;
  metric: AlertMetric;
  threshold: number;
  webhook_url: string;
  enabled: boolean;
  last_triggered_at: string | null;
  last_value: number | null;
  created_at: string;
}

export interface CreateAlertRuleInput {
  metric: AlertMetric;
  threshold: number;
  webhook_url: string;
}

export interface HbaRule {
  line: number;
  type: string;
  database: string;
  user_name: string;
  address: string;
  method: string;
  raw: string;
}

export interface AddHbaRuleInput {
  type: string;
  database: string;
  user_name: string;
  address: string;
  method: string;
}

export interface RetentionPolicy {
  id: string;
  server_id: string;
  database_name: string;
  schema_name: string;
  table_name: string;
  date_column: string;
  max_age_days: number;
  action: "archive" | "delete";
  enabled: boolean;
  last_run_at: string | null;
  last_run_rows_affected: number | null;
  last_run_error: string;
  created_at: string;
}

export interface CreateRetentionPolicyInput {
  database_name: string;
  schema_name: string;
  table_name: string;
  date_column: string;
  max_age_days: number;
  action: "archive" | "delete";
}

export interface InfraContainer {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  ports: string[];
  networks: string[];
  labels: Record<string, string>;
  project?: string;
}

export interface CreateContainerFromImageInput {
  name: string;
  image: string;
  env: Record<string, string>;
  ports: Record<string, number>;
  network?: string;
}

export interface InfraNetwork {
  id: string;
  name: string;
  driver: string;
  scope: string;
}

export interface InfraVolume {
  name: string;
  driver: string;
  mountpoint: string;
  size_bytes?: number;
}

export type BackupStorageKind = "local" | "gdrive";

export interface Backup {
  id: string;
  server_id: string;
  policy_id?: string;
  database_name: string;
  storage: BackupStorageKind;
  filename: string;
  size_bytes?: number;
  status: "running" | "completed" | "failed";
  error?: string;
  started_at: string;
  completed_at?: string;
}

export interface BackupPolicy {
  id: string;
  server_id: string;
  database_name: string;
  storage: BackupStorageKind;
  frequency: "daily" | "weekly";
  weekday?: number;
  time_of_day: string;
  retention_count: number;
  enabled: boolean;
  last_run_at: string | null;
  last_run_status: string;
  last_run_error: string;
  created_at: string;
}

export interface CreateBackupPolicyInput {
  database_name: string;
  storage: BackupStorageKind;
  frequency: "daily" | "weekly";
  weekday?: number | null;
  time_of_day: string;
  retention_count: number;
}

export interface RestoreBackupInput {
  target_database?: string;
  create_new?: boolean;
  new_database_name?: string;
}

export interface GDriveStatus {
  configured: boolean;
  connected: boolean;
  account_email?: string;
  connected_at?: string;
}

export interface TuningSuggestion {
  param: string;
  current_value: string;
  suggested_value: string;
  reason: string;
  differs: boolean;
}

export interface IndexSuggestion {
  schema: string;
  table: string;
  seq_scan: number;
  seq_tup_read: number;
  idx_scan: number;
  live_rows: number;
  detail: string;
}

export interface UnusedIndex {
  schema: string;
  table: string;
  name: string;
  size_bytes: number;
}

export interface CapacityForecast {
  current_disk_mb: number;
  disk_limit_mb: number;
  trend_mb_per_day: number;
  days_until_full: number | null;
  sample_window: string;
}

export interface TriggerInfo {
  name: string;
  schema: string;
  table: string;
  function_name: string;
  enabled: boolean;
  definition: string;
}

export interface CreateTriggerInput {
  name: string;
  schema: string;
  table: string;
  timing: "BEFORE" | "AFTER" | "INSTEAD OF";
  events: ("INSERT" | "UPDATE" | "DELETE" | "TRUNCATE")[];
  level: "ROW" | "STATEMENT";
  function_name: string;
}

export const COLUMN_TYPES = [
  "text",
  "varchar",
  "integer",
  "bigint",
  "smallint",
  "serial",
  "bigserial",
  "boolean",
  "timestamp",
  "timestamptz",
  "date",
  "numeric",
  "real",
  "double precision",
  "uuid",
  "jsonb",
  "json",
] as const;

class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // corpo não era JSON, mantém statusText
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export const api = {
  listServers: () => request<ManagedServer[]>("/api/v1/servers"),
  getServer: (id: string) => request<ManagedServer>(`/api/v1/servers/${id}`),
  createServer: (input: CreateServerInput) =>
    request<ManagedServer>("/api/v1/servers", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  updateServer: (id: string, input: UpdateServerInput) =>
    request<ManagedServer>(`/api/v1/servers/${id}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    }),

  startServer: (id: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/start`, { method: "POST" }),
  stopServer: (id: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/stop`, { method: "POST" }),
  restartServer: (id: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/restart`, { method: "POST" }),
  deleteServer: (id: string, keepVolume: boolean) =>
    request<void>(`/api/v1/servers/${id}?keep_volume=${keepVolume}`, {
      method: "DELETE",
    }),

  platformStats: () => request<PlatformStats>(`/api/v1/platform-stats`),

  platformStatsHistory: () => request<PlatformMetricPoint[]>(`/api/v1/platform-stats-history`),

  discover: () => request<DiscoveredContainer[]>(`/api/v1/discover`),

  registerDiscovered: (containerId: string, input: RegisterDiscoveredInput) =>
    request<ManagedServer>(`/api/v1/discover/${containerId}/register`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  getPassword: (id: string) => request<{ password: string }>(`/api/v1/servers/${id}/password`),

  rotateSuperuserPassword: (id: string) =>
    request<{ password: string }>(`/api/v1/servers/${id}/password/rotate`, { method: "POST" }),

  rotateRolePassword: (id: string, name: string) =>
    request<{ password: string }>(
      `/api/v1/servers/${id}/roles/${encodeURIComponent(name)}/rotate-password`,
      { method: "POST" }
    ),

  slowQueries: (id: string, database: string, orderBy: "total_time" | "mean_time" | "calls") =>
    request<{ available: boolean; queries: SlowQuery[] }>(
      `/api/v1/servers/${id}/slow-queries?database=${encodeURIComponent(database)}&order_by=${orderBy}`
    ),

  resetQueryStats: (id: string, database: string) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/slow-queries/reset?database=${encodeURIComponent(database)}`,
      { method: "POST" }
    ),

  enableQueryStats: (id: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/query-stats/enable`, { method: "POST" }),

  listRoles: (id: string) => request<RoleInfo[]>(`/api/v1/servers/${id}/roles`),

  createRole: (id: string, input: CreateRoleInput) =>
    request<{ status: string }>(`/api/v1/servers/${id}/roles`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  dropRole: (id: string, name: string) =>
    request<void>(`/api/v1/servers/${id}/roles/${encodeURIComponent(name)}`, { method: "DELETE" }),

  rolePrivileges: (id: string, name: string, database: string) =>
    request<TablePrivileges[]>(
      `/api/v1/servers/${id}/roles/${encodeURIComponent(name)}/privileges?database=${encodeURIComponent(database)}`
    ),

  setPrivilege: (
    id: string,
    name: string,
    database: string,
    schema: string,
    table: string,
    privilege: Privilege,
    grant: boolean
  ) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/roles/${encodeURIComponent(name)}/privileges/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/${privilege.toUpperCase()}/${grant ? "grant" : "revoke"}?database=${encodeURIComponent(database)}`,
      { method: "POST" }
    ),

  listDatabases: (id: string) => request<string[]>(`/api/v1/servers/${id}/databases`),

  createDatabase: (id: string, name: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/databases`, {
      method: "POST",
      body: JSON.stringify({ name }),
    }),

  dropDatabase: (id: string, name: string) =>
    request<void>(`/api/v1/servers/${id}/databases/${encodeURIComponent(name)}`, { method: "DELETE" }),

  databaseSizes: (id: string) => request<DatabaseSize[]>(`/api/v1/servers/${id}/database-sizes`),

  metricsHistory: (id: string) => request<MetricPoint[]>(`/api/v1/servers/${id}/metrics-history`),

  listTables: (id: string, database: string) =>
    request<TableInfo[]>(`/api/v1/servers/${id}/tables?database=${encodeURIComponent(database)}`),

  createTable: (id: string, database: string, input: CreateTableInput) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/tables?database=${encodeURIComponent(database)}`,
      { method: "POST", body: JSON.stringify(input) }
    ),

  dropTable: (id: string, database: string, schema: string, table: string) =>
    request<void>(
      `/api/v1/servers/${id}/tables/${encodeURIComponent(schema)}/${encodeURIComponent(table)}?database=${encodeURIComponent(database)}`,
      { method: "DELETE" }
    ),

  tableRows: (
    id: string,
    schema: string,
    table: string,
    opts: { database: string; limit?: number; offset?: number }
  ) => {
    const params = new URLSearchParams({
      database: opts.database,
      limit: String(opts.limit ?? 50),
      offset: String(opts.offset ?? 0),
    });
    return request<TableRowsResult>(
      `/api/v1/servers/${id}/tables/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/rows?${params}`
    );
  },

  runQuery: (id: string, database: string, sql: string) =>
    request<QueryResult>(`/api/v1/servers/${id}/query`, {
      method: "POST",
      body: JSON.stringify({ database, sql }),
    }),

  explainQuery: (id: string, database: string, sql: string, analyze: boolean) =>
    request<ExplainResult>(`/api/v1/servers/${id}/explain`, {
      method: "POST",
      body: JSON.stringify({ database, sql, analyze }),
    }),

  activity: (id: string, database: string) =>
    request<ActivitySession[]>(
      `/api/v1/servers/${id}/activity?database=${encodeURIComponent(database)}`
    ),

  cancelBackend: (id: string, pid: number) =>
    request<{ status: string }>(`/api/v1/servers/${id}/activity/${pid}/cancel`, {
      method: "POST",
    }),

  terminateBackend: (id: string, pid: number) =>
    request<{ status: string }>(`/api/v1/servers/${id}/activity/${pid}/terminate`, {
      method: "POST",
    }),

  logs: (id: string, tail = 500) =>
    request<{ logs: string }>(`/api/v1/servers/${id}/logs?tail=${tail}`),

  logsTimeline: (id: string, tail = 200) =>
    request<LogLine[]>(`/api/v1/servers/${id}/logs-timeline?tail=${tail}`),

  stats: (id: string) => request<ContainerStats>(`/api/v1/servers/${id}/stats`),

  listTriggers: (id: string, database: string, schema: string, table: string) =>
    request<TriggerInfo[]>(
      `/api/v1/servers/${id}/triggers?database=${encodeURIComponent(database)}&schema=${encodeURIComponent(schema)}&table=${encodeURIComponent(table)}`
    ),

  listTriggerFunctions: (id: string, database: string) =>
    request<string[]>(`/api/v1/servers/${id}/trigger-functions?database=${encodeURIComponent(database)}`),

  createTrigger: (id: string, database: string, input: CreateTriggerInput) =>
    request<{ status: string }>(`/api/v1/servers/${id}/triggers?database=${encodeURIComponent(database)}`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  setTriggerEnabled: (
    id: string,
    database: string,
    schema: string,
    table: string,
    name: string,
    enabled: boolean
  ) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/triggers/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/${encodeURIComponent(name)}/${enabled ? "enable" : "disable"}?database=${encodeURIComponent(database)}`,
      { method: "POST" }
    ),

  dropTrigger: (id: string, database: string, schema: string, table: string, name: string) =>
    request<void>(
      `/api/v1/servers/${id}/triggers/${encodeURIComponent(schema)}/${encodeURIComponent(table)}/${encodeURIComponent(name)}?database=${encodeURIComponent(database)}`,
      { method: "DELETE" }
    ),

  listHbaRules: (id: string) => request<HbaRule[]>(`/api/v1/servers/${id}/hba-rules`),

  addHbaRule: (id: string, input: AddHbaRuleInput) =>
    request<{ status: string }>(`/api/v1/servers/${id}/hba-rules`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  deleteHbaRule: (id: string, raw: string) =>
    request<void>(`/api/v1/servers/${id}/hba-rules/delete`, {
      method: "POST",
      body: JSON.stringify({ raw }),
    }),

  enablePooling: (id: string, poolMode: string) =>
    request<ManagedServer>(`/api/v1/servers/${id}/pooling/enable`, {
      method: "POST",
      body: JSON.stringify({ pool_mode: poolMode }),
    }),

  disablePooling: (id: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/pooling/disable`, {
      method: "POST",
    }),

  getConfig: (id: string, database: string) =>
    request<LiveParam[]>(`/api/v1/servers/${id}/config?database=${encodeURIComponent(database)}`),

  updateConfig: (id: string, database: string, updates: Record<string, string>) =>
    request<{ restart_required: boolean }>(
      `/api/v1/servers/${id}/config?database=${encodeURIComponent(database)}`,
      { method: "PUT", body: JSON.stringify({ updates }) }
    ),

  listExtensions: (id: string, database: string) =>
    request<Extension[]>(`/api/v1/servers/${id}/extensions?database=${encodeURIComponent(database)}`),

  enableExtension: (id: string, database: string, name: string) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/extensions/${encodeURIComponent(name)}/enable?database=${encodeURIComponent(database)}`,
      { method: "POST" }
    ),

  disableExtension: (id: string, database: string, name: string) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/extensions/${encodeURIComponent(name)}/disable?database=${encodeURIComponent(database)}`,
      { method: "POST" }
    ),

  listViews: (id: string, database: string) =>
    request<ViewInfo[]>(`/api/v1/servers/${id}/views?database=${encodeURIComponent(database)}`),
  createView: (id: string, database: string, schema: string, name: string, query: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/views?database=${encodeURIComponent(database)}`, {
      method: "POST",
      body: JSON.stringify({ schema, name, query }),
    }),
  dropView: (id: string, database: string, schema: string, name: string) =>
    request<void>(
      `/api/v1/servers/${id}/views/${encodeURIComponent(schema)}/${encodeURIComponent(name)}?database=${encodeURIComponent(database)}`,
      { method: "DELETE" }
    ),

  listMaterializedViews: (id: string, database: string) =>
    request<MaterializedViewInfo[]>(
      `/api/v1/servers/${id}/materialized-views?database=${encodeURIComponent(database)}`
    ),
  createMaterializedView: (id: string, database: string, schema: string, name: string, query: string) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/materialized-views?database=${encodeURIComponent(database)}`,
      { method: "POST", body: JSON.stringify({ schema, name, query }) }
    ),
  refreshMaterializedView: (id: string, database: string, schema: string, name: string) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/materialized-views/${encodeURIComponent(schema)}/${encodeURIComponent(name)}/refresh?database=${encodeURIComponent(database)}`,
      { method: "POST" }
    ),
  dropMaterializedView: (id: string, database: string, schema: string, name: string) =>
    request<void>(
      `/api/v1/servers/${id}/materialized-views/${encodeURIComponent(schema)}/${encodeURIComponent(name)}?database=${encodeURIComponent(database)}`,
      { method: "DELETE" }
    ),

  listSequences: (id: string, database: string) =>
    request<SequenceInfo[]>(`/api/v1/servers/${id}/sequences?database=${encodeURIComponent(database)}`),
  createSequence: (id: string, database: string, input: CreateSequenceInput) =>
    request<{ status: string }>(`/api/v1/servers/${id}/sequences?database=${encodeURIComponent(database)}`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  dropSequence: (id: string, database: string, schema: string, name: string) =>
    request<void>(
      `/api/v1/servers/${id}/sequences/${encodeURIComponent(schema)}/${encodeURIComponent(name)}?database=${encodeURIComponent(database)}`,
      { method: "DELETE" }
    ),

  listTypes: (id: string, database: string) =>
    request<TypeInfo[]>(`/api/v1/servers/${id}/types?database=${encodeURIComponent(database)}`),
  createEnumType: (id: string, database: string, schema: string, name: string, values: string[]) =>
    request<{ status: string }>(`/api/v1/servers/${id}/types/enum?database=${encodeURIComponent(database)}`, {
      method: "POST",
      body: JSON.stringify({ schema, name, values }),
    }),
  createDomain: (
    id: string,
    database: string,
    schema: string,
    name: string,
    baseType: string,
    checkExpr: string
  ) =>
    request<{ status: string }>(`/api/v1/servers/${id}/types/domain?database=${encodeURIComponent(database)}`, {
      method: "POST",
      body: JSON.stringify({ schema, name, base_type: baseType, check_expr: checkExpr }),
    }),
  dropType: (id: string, database: string, schema: string, name: string) =>
    request<void>(
      `/api/v1/servers/${id}/types/${encodeURIComponent(schema)}/${encodeURIComponent(name)}?database=${encodeURIComponent(database)}`,
      { method: "DELETE" }
    ),

  listFunctions: (id: string, database: string) =>
    request<FunctionInfo[]>(`/api/v1/servers/${id}/functions?database=${encodeURIComponent(database)}`),
  createFunction: (id: string, database: string, sql: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/functions?database=${encodeURIComponent(database)}`, {
      method: "POST",
      body: JSON.stringify({ sql }),
    }),
  dropFunction: (id: string, database: string, schema: string, name: string, identityArgs: string) =>
    request<void>(
      `/api/v1/servers/${id}/functions/${encodeURIComponent(schema)}/${encodeURIComponent(name)}?database=${encodeURIComponent(database)}&identity_args=${encodeURIComponent(identityArgs)}`,
      { method: "DELETE" }
    ),

  listBloat: (id: string, database: string) =>
    request<TableBloat[]>(`/api/v1/servers/${id}/bloat?database=${encodeURIComponent(database)}`),

  wraparoundStatus: (id: string) => request<WraparoundInfo[]>(`/api/v1/servers/${id}/wraparound`),

  healthScore: (id: string, database: string) =>
    request<HealthScore>(`/api/v1/servers/${id}/health-score?database=${encodeURIComponent(database)}`),

  capacityForecast: (id: string) =>
    request<CapacityForecast>(`/api/v1/servers/${id}/capacity-forecast`),

  tuningSuggestions: (id: string) => request<TuningSuggestion[]>(`/api/v1/servers/${id}/tuning-suggestions`),

  listRetentionPolicies: (id: string) =>
    request<RetentionPolicy[]>(`/api/v1/servers/${id}/retention-policies`),

  createRetentionPolicy: (id: string, input: CreateRetentionPolicyInput) =>
    request<RetentionPolicy>(`/api/v1/servers/${id}/retention-policies`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  deleteRetentionPolicy: (id: string, policyId: string) =>
    request<void>(`/api/v1/servers/${id}/retention-policies/${policyId}`, { method: "DELETE" }),

  setRetentionPolicyEnabled: (id: string, policyId: string, enabled: boolean) =>
    request<{ status: string }>(`/api/v1/servers/${id}/retention-policies/${policyId}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    }),

  runRetentionPolicy: (id: string, policyId: string) =>
    request<{ rows_affected: number }>(`/api/v1/servers/${id}/retention-policies/${policyId}/run`, {
      method: "POST",
    }),

  listAlertRules: (id: string) => request<AlertRule[]>(`/api/v1/servers/${id}/alert-rules`),

  createAlertRule: (id: string, input: CreateAlertRuleInput) =>
    request<AlertRule>(`/api/v1/servers/${id}/alert-rules`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  deleteAlertRule: (id: string, ruleId: string) =>
    request<void>(`/api/v1/servers/${id}/alert-rules/${ruleId}`, { method: "DELETE" }),

  setAlertRuleEnabled: (id: string, ruleId: string, enabled: boolean) =>
    request<{ status: string }>(`/api/v1/servers/${id}/alert-rules/${ruleId}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    }),

  suggestIndexes: (id: string, database: string) =>
    request<IndexSuggestion[]>(`/api/v1/servers/${id}/indexes/suggestions?database=${encodeURIComponent(database)}`),

  unusedIndexes: (id: string, database: string) =>
    request<UnusedIndex[]>(`/api/v1/servers/${id}/indexes/unused?database=${encodeURIComponent(database)}`),

  reindexConcurrently: (id: string, database: string, schema: string, name: string) =>
    request<{ status: string }>(
      `/api/v1/servers/${id}/indexes/${encodeURIComponent(schema)}/${encodeURIComponent(name)}/reindex-concurrently?database=${encodeURIComponent(database)}`,
      { method: "POST" }
    ),

  dropIndex: (id: string, database: string, schema: string, name: string) =>
    request<void>(
      `/api/v1/servers/${id}/indexes/${encodeURIComponent(schema)}/${encodeURIComponent(name)}?database=${encodeURIComponent(database)}`,
      { method: "DELETE" }
    ),

  listBackups: (id: string) => request<Backup[]>(`/api/v1/servers/${id}/backups`),

  createBackup: (id: string, database: string, storage: BackupStorageKind) =>
    request<Backup>(`/api/v1/servers/${id}/backups`, {
      method: "POST",
      body: JSON.stringify({ database, storage }),
    }),

  deleteBackup: (id: string, backupId: string) =>
    request<void>(`/api/v1/servers/${id}/backups/${backupId}`, { method: "DELETE" }),

  downloadBackupUrl: (id: string, backupId: string) =>
    `${API_URL}/api/v1/servers/${id}/backups/${backupId}/download`,

  restoreBackup: (id: string, backupId: string, input: RestoreBackupInput) =>
    request<{ status: string }>(`/api/v1/servers/${id}/backups/${backupId}/restore`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  listBackupPolicies: (id: string) => request<BackupPolicy[]>(`/api/v1/servers/${id}/backup-policies`),

  createBackupPolicy: (id: string, input: CreateBackupPolicyInput) =>
    request<BackupPolicy>(`/api/v1/servers/${id}/backup-policies`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  deleteBackupPolicy: (id: string, policyId: string) =>
    request<void>(`/api/v1/servers/${id}/backup-policies/${policyId}`, { method: "DELETE" }),

  setBackupPolicyEnabled: (id: string, policyId: string, enabled: boolean) =>
    request<{ status: string }>(`/api/v1/servers/${id}/backup-policies/${policyId}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    }),

  runBackupPolicy: (id: string, policyId: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}/backup-policies/${policyId}/run`, {
      method: "POST",
    }),

  gdriveStatus: () => request<GDriveStatus>(`/api/v1/gdrive/status`),

  setGDriveConfig: (clientId: string, clientSecret: string) =>
    request<{ status: string }>(`/api/v1/gdrive/config`, {
      method: "POST",
      body: JSON.stringify({ client_id: clientId, client_secret: clientSecret }),
    }),

  gdriveAuthUrl: () => request<{ url: string }>(`/api/v1/gdrive/auth-url`),

  gdriveDisconnect: () => request<{ status: string }>(`/api/v1/gdrive/disconnect`, { method: "POST" }),

  listInfraContainers: () => request<InfraContainer[]>(`/api/v1/infra/containers`),

  createInfraContainer: (input: CreateContainerFromImageInput) =>
    request<{ id: string }>(`/api/v1/infra/containers`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  startInfraContainer: (id: string) =>
    request<{ status: string }>(`/api/v1/infra/containers/${id}/start`, { method: "POST" }),

  stopInfraContainer: (id: string) =>
    request<{ status: string }>(`/api/v1/infra/containers/${id}/stop`, { method: "POST" }),

  restartInfraContainer: (id: string) =>
    request<{ status: string }>(`/api/v1/infra/containers/${id}/restart`, { method: "POST" }),

  removeInfraContainer: (id: string) =>
    request<void>(`/api/v1/infra/containers/${id}`, { method: "DELETE" }),

  infraContainerLogs: (id: string, tail = 500) =>
    request<{ logs: string }>(`/api/v1/infra/containers/${id}/logs?tail=${tail}`),

  listInfraNetworks: () => request<InfraNetwork[]>(`/api/v1/infra/networks`),

  createInfraNetwork: (name: string) =>
    request<{ status: string }>(`/api/v1/infra/networks`, {
      method: "POST",
      body: JSON.stringify({ name }),
    }),

  removeInfraNetwork: (id: string) =>
    request<void>(`/api/v1/infra/networks/${id}`, { method: "DELETE" }),

  listInfraVolumes: () => request<InfraVolume[]>(`/api/v1/infra/volumes`),

  createInfraVolume: (name: string) =>
    request<{ status: string }>(`/api/v1/infra/volumes`, {
      method: "POST",
      body: JSON.stringify({ name }),
    }),

  removeInfraVolume: (name: string) =>
    request<void>(`/api/v1/infra/volumes/${name}`, { method: "DELETE" }),
};

export { ApiError };
