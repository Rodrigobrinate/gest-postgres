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
};

export { ApiError };
