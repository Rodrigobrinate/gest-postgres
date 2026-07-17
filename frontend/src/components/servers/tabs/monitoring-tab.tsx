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
import { Badge } from "@/components/ui/badge";
import { Square, XCircle, Database, AlertTriangle } from "lucide-react";
import { MetricChart } from "../metric-chart";
import { cn } from "@/lib/utils";

function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  return `${(bytes / 1024 ** 2).toFixed(0)} MB`;
}

function healthColor(score: number) {
  if (score >= 80) return "text-emerald-600";
  if (score >= 50) return "text-amber-600";
  return "text-red-600";
}

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

  const { data: history } = useQuery({
    queryKey: ["servers", serverId, "metrics-history"],
    queryFn: () => api.metricsHistory(serverId),
    refetchInterval: 15_000,
  });

  const { data: dbSizes } = useQuery({
    queryKey: ["servers", serverId, "database-sizes"],
    queryFn: () => api.databaseSizes(serverId),
  });

  const { data: health } = useQuery({
    queryKey: ["servers", serverId, "health-score", database],
    queryFn: () => api.healthScore(serverId, database),
    enabled: !!database,
    refetchInterval: 30_000,
  });

  const { data: bloat } = useQuery({
    queryKey: ["servers", serverId, "bloat", database],
    queryFn: () => api.listBloat(serverId, database),
    enabled: !!database,
  });

  const { data: wraparound } = useQuery({
    queryKey: ["servers", serverId, "wraparound"],
    queryFn: () => api.wraparoundStatus(serverId),
  });

  const { data: forecast } = useQuery({
    queryKey: ["servers", serverId, "capacity-forecast"],
    queryFn: () => api.capacityForecast(serverId),
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
      <div className="grid grid-cols-4 gap-4">
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
        <StatCard
          label="Health score"
          value={health ? `${health.score}/100` : "—"}
          valueClassName={health ? healthColor(health.score) : undefined}
          hint={health?.factors.map((f) => `${f.name}: ${f.score}`).join(" · ")}
        />
      </div>

      <div className="grid grid-cols-3 gap-4">
        <MetricChart
          title="CPU"
          data={history ?? []}
          dataKey="cpu_percent"
          color="#2563eb"
          unit="%"
        />
        <MetricChart
          title="Memória"
          data={history ?? []}
          dataKey="memory_used_mb"
          color="#7c3aed"
          formatValue={(v) => `${v.toFixed(0)} MB`}
        />
        <MetricChart
          title="Conexões"
          data={history ?? []}
          dataKey="connection_count"
          color="#0891b2"
          formatValue={(v) => String(Math.round(v))}
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Bancos de dados</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {!dbSizes || dbSizes.length === 0 ? (
              <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
            ) : (
              <ul className="divide-y">
                {dbSizes.map((d) => (
                  <li key={d.name} className="flex items-center justify-between px-4 py-2 text-sm">
                    <span className="flex items-center gap-2 font-mono">
                      <Database className="text-muted-foreground size-3.5" />
                      {d.name}
                    </span>
                    <span className="text-muted-foreground">{formatBytes(d.size_bytes)}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Previsão de capacidade</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-1 text-sm">
            {!forecast ? (
              <p className="text-muted-foreground">Carregando...</p>
            ) : (
              <>
                <p>
                  <span className="font-medium">{formatBytes(forecast.current_disk_mb * 1024 * 1024)}</span>
                  {" de "}
                  {formatBytes(forecast.disk_limit_mb * 1024 * 1024)} usados
                </p>
                {forecast.days_until_full != null ? (
                  <p className={forecast.days_until_full < 14 ? "text-red-600" : "text-muted-foreground"}>
                    Ritmo atual: disco cheio em ~{Math.round(forecast.days_until_full)} dias
                  </p>
                ) : (
                  <p className="text-muted-foreground">
                    {forecast.trend_mb_per_day > 0
                      ? "Crescendo, mas sem risco no ritmo atual"
                      : "Sem tendência de crescimento na janela observada"}
                  </p>
                )}
                <p className="text-muted-foreground text-xs">
                  Baseado em {forecast.sample_window} (histórico em memória, reseta se o backend reiniciar)
                </p>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Wraparound (age de transação)</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {!wraparound || wraparound.length === 0 ? (
              <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
            ) : (
              <ul className="divide-y">
                {wraparound.map((w) => (
                  <li key={w.database} className="flex items-center justify-between px-4 py-2 text-sm">
                    <span className="font-mono">{w.database}</span>
                    <div className="flex items-center gap-2">
                      <span className="text-muted-foreground">{w.age.toLocaleString("pt-BR")}</span>
                      {w.status === "ok" ? (
                        <Badge variant="outline">ok</Badge>
                      ) : (
                        <Badge
                          className={cn(
                            "gap-1",
                            w.status === "critical" ? "bg-red-600 text-white" : "bg-amber-500 text-white"
                          )}
                        >
                          <AlertTriangle className="size-3" />
                          {w.status === "critical" ? "crítico" : "atenção"}
                        </Badge>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Bloat (tuplas mortas)</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {!bloat || bloat.length === 0 ? (
              <p className="text-muted-foreground p-6 text-sm">Nenhuma tabela com dados ainda.</p>
            ) : (
              <ul className="divide-y">
                {bloat.slice(0, 8).map((b) => (
                  <li key={`${b.schema}.${b.table}`} className="px-4 py-2 text-sm">
                    <div className="flex items-center justify-between">
                      <span className="font-mono text-xs">
                        {b.schema}.{b.table}
                      </span>
                      <span
                        className={cn(
                          "text-xs",
                          b.dead_ratio >= 0.2 ? "text-red-600" : "text-muted-foreground"
                        )}
                      >
                        {(b.dead_ratio * 100).toFixed(0)}% mortas ({b.dead_tuples})
                      </span>
                    </div>
                    {b.suggestion && (
                      <p className="text-muted-foreground text-xs">{b.suggestion}</p>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
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

function StatCard({
  label,
  value,
  hint,
  valueClassName,
}: {
  label: string;
  value: string;
  hint?: string;
  valueClassName?: string;
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-muted-foreground text-xs">{label}</p>
        <p className={cn("text-2xl font-semibold", valueClassName)}>{value}</p>
        {hint && <p className="text-muted-foreground truncate text-xs" title={hint}>{hint}</p>}
      </CardContent>
    </Card>
  );
}
