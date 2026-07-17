"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Square, XCircle } from "lucide-react";

export function MonitoringTab({ serverId, database }: { serverId: string; database: string }) {
  const { data: stats } = useQuery({
    queryKey: ["servers", serverId, "stats"],
    queryFn: () => api.stats(serverId),
  });

  const { data: sessions, isLoading } = useQuery({
    queryKey: ["servers", serverId, "activity", database],
    queryFn: () => api.activity(serverId, database),
    enabled: !!database,
  });

  const queryClient = useQueryClient();
  const invalidateActivity = () =>
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "activity"] });

  const cancel = useMutation({
    mutationFn: (pid: number) => api.cancelBackend(serverId, pid),
    onSuccess: () => {
      toast.success("Query cancelada");
      invalidateActivity();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao cancelar"),
  });

  const terminate = useMutation({
    mutationFn: (pid: number) => api.terminateBackend(serverId, pid),
    onSuccess: () => {
      toast.success("Sessão encerrada");
      invalidateActivity();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao encerrar"),
  });

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-3 gap-4">
        <StatCard label="CPU" value={stats ? `${stats.cpu_percent.toFixed(1)}%` : "—"} />
        <StatCard
          label="Memória"
          value={stats ? `${stats.memory_used_mb.toFixed(0)} MB` : "—"}
          hint={
            stats
              ? `de ${stats.memory_limit_mb.toFixed(0)} MB (${stats.memory_percent.toFixed(0)}%)`
              : undefined
          }
        />
        <StatCard label="Sessões ativas" value={sessions ? String(sessions.length) : "—"} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Sessões (pg_stat_activity)</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !sessions || sessions.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhuma sessão ativa.</p>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>PID</TableHead>
                    <TableHead>Usuário</TableHead>
                    <TableHead>Banco</TableHead>
                    <TableHead>Estado</TableHead>
                    <TableHead>Query</TableHead>
                    <TableHead className="text-right">Ações</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sessions.map((s) => (
                    <TableRow key={s.pid}>
                      <TableCell className="font-mono text-xs">{s.pid}</TableCell>
                      <TableCell>{s.username || "—"}</TableCell>
                      <TableCell>{s.database || "—"}</TableCell>
                      <TableCell>
                        <span
                          className={
                            s.state === "active" ? "text-emerald-600" : "text-muted-foreground"
                          }
                        >
                          {s.state || "—"}
                        </span>
                      </TableCell>
                      <TableCell className="max-w-xs truncate font-mono text-xs" title={s.query}>
                        {s.query || "—"}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-1">
                          <Button
                            size="icon"
                            variant="ghost"
                            title="Cancelar query"
                            disabled={cancel.isPending}
                            onClick={() => cancel.mutate(s.pid)}
                          >
                            <Square className="size-4" />
                          </Button>
                          <Button
                            size="icon"
                            variant="ghost"
                            title="Encerrar sessão"
                            className="text-red-600 hover:text-red-700"
                            disabled={terminate.isPending}
                            onClick={() => terminate.mutate(s.pid)}
                          >
                            <XCircle className="size-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-muted-foreground text-xs">{label}</p>
        <p className="text-2xl font-semibold">{value}</p>
        {hint && <p className="text-muted-foreground text-xs">{hint}</p>}
      </CardContent>
    </Card>
  );
}
