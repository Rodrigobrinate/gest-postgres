import { API_URL } from "@/lib/api";

// Chamadas nativas do Worker do sistema mestre (Cloudflare) — NUNCA passam
// por apiPath/proxy: são rotas do próprio Worker (listar instalação
// cadastrada), não do backend de um droplet específico. Só existem em
// MULTI_SERVER_MODE; fora dele (build normal por droplet) essas funções
// nunca são chamadas.
export interface MasterServerSummary {
  id: string;
  name: string;
  tunnel_hostname: string;
  online: boolean;
  version?: string;
}

async function masterRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // corpo não era JSON, mantém statusText
    }
    throw new Error(message);
  }
  return res.json() as Promise<T>;
}

export interface CreateMasterServerInput {
  name: string;
  tunnel_hostname: string;
}

// integration_key só existe nessa resposta — o Worker GERA a chave (nunca
// aceita uma vinda do cliente), mostrada uma vez pra colar no
// `setup.sh --integration-key` do droplet.
export interface CreateMasterServerResult {
  id: string;
  integration_key: string;
}

export interface UpdateMasterServerInput {
  name: string;
  tunnel_hostname: string;
}

export const masterApi = {
  listServers: () => masterRequest<MasterServerSummary[]>("/servers"),
  createServer: (input: CreateMasterServerInput) =>
    masterRequest<CreateMasterServerResult>("/servers", { method: "POST", body: JSON.stringify(input) }),
  updateServer: (id: string, input: UpdateMasterServerInput) =>
    masterRequest<{ ok: boolean }>(`/servers/${id}`, { method: "PATCH", body: JSON.stringify(input) }),
  deleteServer: (id: string) => masterRequest<{ ok: boolean }>(`/servers/${id}`, { method: "DELETE" }),
};
