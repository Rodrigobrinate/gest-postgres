"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { masterApi } from "@/lib/master-api";
import { useSelectedServer } from "@/lib/server-context";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { LogoutButton } from "@/components/auth/logout-button";
import { CreateInstallationDialog } from "./create-installation-dialog";
import { EditInstallationDialog } from "./edit-installation-dialog";
import { Database, Trash2, Radar } from "lucide-react";
import type { MasterServerStats, PingResult } from "@/lib/master-api";

function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(0)} MB`;
  return `${bytes} B`;
}

function StatsRow({ stats }: { stats: MasterServerStats }) {
  const memPercent =
    stats.total_memory_limit_mb && stats.total_memory_used_mb
      ? Math.round((stats.total_memory_used_mb / stats.total_memory_limit_mb) * 100)
      : undefined;
  const diskPercent =
    stats.disk_total_bytes && stats.disk_used_bytes
      ? Math.round((stats.disk_used_bytes / stats.disk_total_bytes) * 100)
      : undefined;
  return (
    <div className="text-muted-foreground grid grid-cols-3 gap-2 text-xs">
      <div>
        <div className="text-foreground font-medium">
          {stats.total_cpu_percent !== undefined ? `${stats.total_cpu_percent.toFixed(1)}%` : "—"}
        </div>
        <div>CPU</div>
      </div>
      <div>
        <div className="text-foreground font-medium">{memPercent !== undefined ? `${memPercent}%` : "—"}</div>
        <div>Memória</div>
      </div>
      <div>
        <div className="text-foreground font-medium">{diskPercent !== undefined ? `${diskPercent}%` : "—"}</div>
        <div>Disco{stats.disk_total_bytes ? ` de ${formatBytes(stats.disk_total_bytes)}` : ""}</div>
      </div>
    </div>
  );
}

// Tela inicial em MULTI_SERVER_MODE (hospedado no Cloudflare Pages, atrás
// do Worker do sistema mestre) enquanto nenhuma instalação foi selecionada
// — métricas básicas de TODAS as instalações gest-postgres cadastradas,
// clicar numa entra no dashboard de sempre (intocado) escopado só àquela
// instalação. "Instalação" aqui = um droplet/servidor gest-postgres inteiro
// (backend + Postgres gerenciados dele), não confundir com a lista de
// servidores Postgres que já existe DENTRO de cada instalação.
export function InstallationsOverview() {
  const { selectServer } = useSelectedServer();
  const queryClient = useQueryClient();
  const { data, isLoading, error } = useQuery({
    queryKey: ["master-servers"],
    queryFn: masterApi.listServers,
    // Explícito mesmo já sendo o default global (providers.tsx) — cada
    // busca dispara uma checagem AO VIVO em toda instalação (ver
    // listInstallations no Worker), então esse intervalo é o que decide de
    // quanto em quanto tempo "online"/métrica básica ficam frescas aqui.
    refetchInterval: 5_000,
    refetchIntervalInBackground: true,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => masterApi.deleteServer(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["master-servers"] });
      toast.success("Instalação removida");
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : "Falha ao remover"),
  });

  // Ping manual — pedido explícito: testar na hora, pela própria UI, sem
  // depender de curl/log externo pra saber o motivo exato de um "offline"
  // (status HTTP, latência, mensagem de erro crua do fetch).
  const pingMutation = useMutation({
    mutationFn: () => masterApi.pingAll(),
    onError: (err) => toast.error(err instanceof Error ? err.message : "Falha ao testar"),
  });
  const pingResults = pingMutation.data;

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 p-6 sm:p-10">
      <header className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="bg-primary text-primary-foreground flex size-10 items-center justify-center rounded-lg">
            <Database className="size-5" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">gest-postgres</h1>
            <p className="text-muted-foreground text-sm">Escolha uma instalação</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => pingMutation.mutate()} disabled={pingMutation.isPending}>
            <Radar className="size-4" />
            {pingMutation.isPending ? "Testando..." : "Testar conexão"}
          </Button>
          <CreateInstallationDialog />
          <LogoutButton />
        </div>
      </header>

      {pingResults && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Resultado do teste</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {pingResults.map((r: PingResult) => (
              <div key={r.id} className="flex items-center justify-between gap-4 text-sm">
                <span className="font-medium">{r.name}</span>
                <span className="text-muted-foreground truncate font-mono text-xs">{r.tunnel_hostname}</span>
                <Badge variant={r.ok ? "default" : "destructive"}>
                  {r.ok ? `${r.status} · ${r.ms}ms` : (r.error ?? `status ${r.status}`)}
                </Badge>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {isLoading && <p className="text-muted-foreground text-sm">Carregando instalações...</p>}
      {error && (
        <p className="text-destructive text-sm">
          Falha ao listar instalações: {error instanceof Error ? error.message : "erro desconhecido"}
        </p>
      )}
      {data?.length === 0 && (
        <p className="text-muted-foreground text-sm">Nenhuma instalação cadastrada ainda.</p>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {data?.map((s) => (
          <Card
            key={s.id}
            className="hover:border-primary cursor-pointer transition-colors"
            onClick={() => selectServer({ id: s.id, name: s.name })}
          >
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-base">{s.name}</CardTitle>
              <div className="flex items-center gap-2">
                <Badge variant={s.online ? "default" : "destructive"}>
                  {s.online ? "online" : "offline"}
                </Badge>
                <EditInstallationDialog installation={s} />
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-6"
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteMutation.mutate(s.id);
                  }}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-3">
              <p className="text-muted-foreground truncate font-mono text-xs">{s.tunnel_hostname}</p>
              {s.online && s.stats && <StatsRow stats={s.stats} />}
              {s.version && <p className="text-muted-foreground text-sm">versão {s.version}</p>}
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
