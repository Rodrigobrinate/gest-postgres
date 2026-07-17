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
  created_at: string;
  updated_at: string;
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

export interface LiveConfig extends PostgresConfig {
  restart_pending: boolean;
}

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

  getPassword: (id: string) => request<{ password: string }>(`/api/v1/servers/${id}/password`),

  listDatabases: (id: string) => request<string[]>(`/api/v1/servers/${id}/databases`),

  listTables: (id: string, database: string) =>
    request<TableInfo[]>(`/api/v1/servers/${id}/tables?database=${encodeURIComponent(database)}`),

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

  stats: (id: string) => request<ContainerStats>(`/api/v1/servers/${id}/stats`),

  getConfig: (id: string, database: string) =>
    request<LiveConfig>(`/api/v1/servers/${id}/config?database=${encodeURIComponent(database)}`),

  updateConfig: (id: string, database: string, cfg: PostgresConfig) =>
    request<{ restart_required: boolean }>(
      `/api/v1/servers/${id}/config?database=${encodeURIComponent(database)}`,
      { method: "PUT", body: JSON.stringify(cfg) }
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
};

export { ApiError };
