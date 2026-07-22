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
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Square, XCircle, Database, AlertTriangle, Plus, Trash2, FlaskConical, Copy } from "lucide-react";
import { MetricChart } from "../metric-chart";
import { DatabaseSizeChart, ConnectionsPerDatabaseChart } from "../database-size-chart";
import { ReadWriteChart } from "../read-write-chart";
import { AlertRules } from "../alert-rules";
import { cn } from "@/lib/utils";
import { useState } from "react";

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

  const [newDbOpen, setNewDbOpen] = useState(false);
  const [newDbName, setNewDbName] = useState("");

  const queryClient = useQueryClient();
  const invalidateDbs = () => {
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "database-sizes"] });
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "databases"] });
  };

  const createDb = useMutation({
    mutationFn: () => api.createDatabase(serverId, newDbName),
    onSuccess: () => {
      toast.success(`Banco "${newDbName}" criado`);
      setNewDbOpen(false);
      setNewDbName("");
      invalidateDbs();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar banco"),
  });

  const dropDb = useMutation({
    mutationFn: (name: string) => api.dropDatabase(serverId, name),
    onSuccess: (_d, name) => {
      toast.success(`Banco "${name}" excluído`);
      invalidateDbs();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir banco"),
  });

  const [testDbOpen, setTestDbOpen] = useState(false);
  const [testDbSuffix, setTestDbSuffix] = useState("");
  const [testDbResult, setTestDbResult] = useState<{
    database: string;
    username: string;
    password: string;
  } | null>(null);
  const createTestDb = useMutation({
    mutationFn: () => api.createTestDatabase(serverId, testDbSuffix.trim()),
    onSuccess: (result) => {
      setTestDbOpen(false);
      setTestDbSuffix("");
      setTestDbResult(result);
      invalidateDbs();
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "roles"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar banco de teste"),
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

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricChart
          title="CPU"
          data={history ?? []}
          dataKey="cpu_percent"
          color="#2563eb"
          unit="%"
          serverId={serverId}
        />
        <MetricChart
          title="Memória"
          data={history ?? []}
          dataKey="memory_used_mb"
          color="#7c3aed"
          formatValue={(v) => `${v.toFixed(0)} MB`}
          serverId={serverId}
        />
        <MetricChart
          title="Conexões"
          data={history ?? []}
          dataKey="connection_count"
          color="#0891b2"
          formatValue={(v) => String(Math.round(v))}
          serverId={serverId}
        />
        <MetricChart
          title="Disco"
          data={history ?? []}
          dataKey="disk_used_mb"
          color="#d97706"
          formatValue={(v) => formatBytes(v * 1024 * 1024)}
          serverId={serverId}
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-base">Bancos de dados</CardTitle>
            <div className="flex gap-1.5">
              <Dialog open={testDbOpen} onOpenChange={setTestDbOpen}>
                <DialogTrigger render={<Button size="sm" variant="outline" />}>
                  <FlaskConical className="size-4" />
                  Criar banco de teste
                </DialogTrigger>
                <DialogContent className="sm:max-w-sm">
                  <DialogHeader>
                    <DialogTitle>Criar banco de teste</DialogTitle>
                  </DialogHeader>
                  <p className="text-muted-foreground text-xs">
                    Cria um banco novo + um usuário com senha, acesso só a esse banco.
                  </p>
                  <div className="flex items-center gap-0 rounded-md border">
                    <span className="bg-muted text-muted-foreground border-r px-2 py-2 font-mono text-sm">
                      test_
                    </span>
                    <Input
                      placeholder="identificador"
                      value={testDbSuffix}
                      onChange={(e) => setTestDbSuffix(e.target.value)}
                      className="border-0 font-mono"
                      autoFocus
                    />
                  </div>
                  <p className="text-muted-foreground text-xs">
                    Só letra, número e underscore, começando com letra ou underscore.
                  </p>
                  <DialogFooter>
                    <Button
                      disabled={
                        createTestDb.isPending || !/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(testDbSuffix.trim())
                      }
                      onClick={() => createTestDb.mutate()}
                    >
                      {createTestDb.isPending ? "Criando..." : "Criar"}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
              <Dialog open={newDbOpen} onOpenChange={setNewDbOpen}>
                <DialogTrigger render={<Button size="sm" variant="outline" />}>
                  <Plus className="size-4" />
                  Novo banco
                </DialogTrigger>
                <DialogContent className="sm:max-w-sm">
                  <DialogHeader>
                    <DialogTitle>Criar banco de dados</DialogTitle>
                  </DialogHeader>
                  <Input
                    placeholder="nome_do_banco"
                    value={newDbName}
                    onChange={(e) => setNewDbName(e.target.value)}
                  />
                  <DialogFooter>
                    <Button
                      disabled={createDb.isPending || !newDbName.trim()}
                      onClick={() => createDb.mutate()}
                    >
                      {createDb.isPending ? "Criando..." : "Criar"}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>
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
                    <div className="flex items-center gap-2">
                      <span className="text-muted-foreground">{formatBytes(d.size_bytes)}</span>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="size-6 text-red-600"
                        disabled={dropDb.isPending}
                        onClick={() => dropDb.mutate(d.name)}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </div>
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

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <DatabaseSizeChart history={history ?? []} serverId={serverId} />
        <ConnectionsPerDatabaseChart history={history ?? []} serverId={serverId} />
        <ReadWriteChart history={history ?? []} serverId={serverId} />
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

      <AlertRules serverId={serverId} />

      <Dialog open={!!testDbResult} onOpenChange={(v) => !v && setTestDbResult(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Banco de teste criado</DialogTitle>
          </DialogHeader>
          <p className="text-muted-foreground text-xs">
            Usuário criado com acesso só a esse banco. A senha não fica guardada na plataforma —
            copie agora, você não vai poder ver de novo.
          </p>
          <div className="grid gap-2 text-sm">
            <Field label="Banco" value={testDbResult?.database ?? ""} />
            <Field label="Usuário" value={testDbResult?.username ?? ""} />
            <Field label="Senha" value={testDbResult?.password ?? ""} />
          </div>
          <DialogFooter>
            <Button
              onClick={() => {
                if (testDbResult) {
                  navigator.clipboard.writeText(
                    `banco: ${testDbResult.database}\nusuário: ${testDbResult.username}\nsenha: ${testDbResult.password}`
                  );
                  toast.success("Copiado");
                }
              }}
            >
              <Copy className="size-4" />
              Copiar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-muted-foreground text-xs">{label}</span>
      <code className="bg-muted rounded border px-2 py-1 font-mono text-xs break-all">{value}</code>
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
