"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { RotateCcw, Zap, Trash2, Wrench } from "lucide-react";

function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  return `${(bytes / 1024 ** 2).toFixed(0)} MB`;
}

const ORDER_OPTIONS = [
  { value: "total_time", label: "Tempo total" },
  { value: "mean_time", label: "Tempo médio" },
  { value: "calls", label: "Nº de chamadas" },
] as const;

export function PerformanceTab({ serverId, database }: { serverId: string; database: string }) {
  const [orderBy, setOrderBy] = useState<(typeof ORDER_OPTIONS)[number]["value"]>("total_time");
  const [search, setSearch] = useState("");
  const [minMeanMs, setMinMeanMs] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["servers", serverId, "slow-queries", database, orderBy],
    queryFn: () => api.slowQueries(serverId, database, orderBy),
    enabled: !!database,
  });

  const queryClient = useQueryClient();
  const reset = useMutation({
    mutationFn: () => api.resetQueryStats(serverId, database),
    onSuccess: () => {
      toast.success("Estatísticas zeradas");
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "slow-queries"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao resetar"),
  });

  const enable = useMutation({
    mutationFn: () => api.enableQueryStats(serverId),
    onSuccess: () => {
      toast.success("Monitoramento de queries habilitado — servidor reiniciou");
      queryClient.invalidateQueries({ queryKey: ["servers", serverId] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao habilitar"),
  });

  const { data: suggestions } = useQuery({
    queryKey: ["servers", serverId, "index-suggestions", database],
    queryFn: () => api.suggestIndexes(serverId, database),
    enabled: !!database,
  });

  const { data: unused } = useQuery({
    queryKey: ["servers", serverId, "unused-indexes", database],
    queryFn: () => api.unusedIndexes(serverId, database),
    enabled: !!database,
  });

  const invalidateUnused = () =>
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "unused-indexes"] });

  const reindex = useMutation({
    mutationFn: (idx: { schema: string; name: string }) =>
      api.reindexConcurrently(serverId, database, idx.schema, idx.name),
    onSuccess: (_d, idx) => {
      toast.success(`${idx.name} reconstruído`);
      invalidateUnused();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao reconstruir índice"),
  });

  const dropIndex = useMutation({
    mutationFn: (idx: { schema: string; name: string }) => api.dropIndex(serverId, database, idx.schema, idx.name),
    onSuccess: (_d, idx) => {
      toast.success(`${idx.name} excluído`);
      invalidateUnused();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir índice"),
  });

  if (!isLoading && data && !data.available) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
          <Zap className="text-muted-foreground size-8" />
          <p className="text-sm font-medium">Monitoramento de queries não habilitado</p>
          <p className="text-muted-foreground max-w-sm text-xs">
            Precisa de <code>pg_stat_statements</code> em <code>shared_preload_libraries</code> —
            isso exige reiniciar o servidor. Um clique faz tudo: configura, reinicia e habilita a
            extensão.
          </p>
          <Button onClick={() => enable.mutate()} disabled={enable.isPending}>
            {enable.isPending ? "Habilitando (reinicia o servidor)..." : "Habilitar agora"}
          </Button>
        </CardContent>
      </Card>
    );
  }

  const minMeanMsNum = Number(minMeanMs);
  const filteredQueries = (data?.queries ?? []).filter((q) => {
    if (search.trim() && !q.query.toLowerCase().includes(search.trim().toLowerCase())) return false;
    if (minMeanMs.trim() && !Number.isNaN(minMeanMsNum) && q.mean_exec_ms < minMeanMsNum) return false;
    return true;
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Select value={orderBy} onValueChange={(v) => v && setOrderBy(v as typeof orderBy)}>
            <SelectTrigger className="w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ORDER_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  Ordenar por: {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Input
            placeholder="Buscar na query..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-56"
          />
          <Input
            type="number"
            placeholder="Tempo médio mín. (ms)"
            value={minMeanMs}
            onChange={(e) => setMinMeanMs(e.target.value)}
            className="w-44"
          />
        </div>
        <Button size="sm" variant="outline" onClick={() => reset.mutate()} disabled={reset.isPending}>
          <RotateCcw className="size-3.5" />
          Zerar estatísticas
        </Button>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !data || data.queries.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">
              Nenhuma query registrada ainda em &ldquo;{database}&rdquo;.
            </p>
          ) : filteredQueries.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhuma query bate com o filtro.</p>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Query</TableHead>
                    <TableHead className="text-right">Chamadas</TableHead>
                    <TableHead className="text-right">Tempo total</TableHead>
                    <TableHead className="text-right">Tempo médio</TableHead>
                    <TableHead className="text-right">Linhas</TableHead>
                    <TableHead className="text-right">Cache hit</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredQueries.map((q) => (
                    <TableRow key={q.query_id}>
                      <TableCell
                        className="max-w-md truncate font-mono text-xs"
                        title={q.query}
                      >
                        {q.query}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs">{q.calls}</TableCell>
                      <TableCell className="text-right font-mono text-xs">
                        {q.total_exec_ms.toFixed(1)}ms
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs">
                        {q.mean_exec_ms.toFixed(2)}ms
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs">{q.rows}</TableCell>
                      <TableCell className="text-right font-mono text-xs">
                        {(q.cache_hit_ratio * 100).toFixed(0)}%
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardContent className="p-0">
            <div className="border-b p-3 text-sm font-medium">Sugestão de índices</div>
            {!suggestions || suggestions.length === 0 ? (
              <p className="text-muted-foreground p-6 text-sm">
                Nenhuma tabela com padrão claro de seq scan alto em relação a uso de índice.
              </p>
            ) : (
              <ul className="divide-y">
                {suggestions.map((s) => (
                  <li key={`${s.schema}.${s.table}`} className="p-3 text-sm">
                    <span className="font-mono text-xs font-medium">
                      {s.schema}.{s.table}
                    </span>
                    <p className="text-muted-foreground text-xs">{s.detail}</p>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-0">
            <div className="border-b p-3 text-sm font-medium">Índices não usados</div>
            {!unused || unused.length === 0 ? (
              <p className="text-muted-foreground p-6 text-sm">Nenhum índice ocioso encontrado.</p>
            ) : (
              <ul className="divide-y">
                {unused.map((u) => (
                  <li key={`${u.schema}.${u.name}`} className="flex items-center justify-between gap-2 p-3 text-sm">
                    <div className="min-w-0">
                      <span className="font-mono text-xs font-medium">{u.name}</span>
                      <p className="text-muted-foreground text-xs">
                        {u.schema}.{u.table} · {formatBytes(u.size_bytes)} · 0 scans
                      </p>
                    </div>
                    <div className="flex shrink-0 gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="REINDEX CONCURRENTLY"
                        disabled={reindex.isPending}
                        onClick={() => reindex.mutate({ schema: u.schema, name: u.name })}
                      >
                        <Wrench className="size-4" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="text-red-600"
                        disabled={dropIndex.isPending}
                        onClick={() => dropIndex.mutate({ schema: u.schema, name: u.name })}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
