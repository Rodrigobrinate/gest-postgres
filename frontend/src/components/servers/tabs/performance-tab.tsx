"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
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
import { RotateCcw, Zap } from "lucide-react";

const ORDER_OPTIONS = [
  { value: "total_time", label: "Tempo total" },
  { value: "mean_time", label: "Tempo médio" },
  { value: "calls", label: "Nº de chamadas" },
] as const;

export function PerformanceTab({ serverId, database }: { serverId: string; database: string }) {
  const [orderBy, setOrderBy] = useState<(typeof ORDER_OPTIONS)[number]["value"]>("total_time");

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

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
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
                  {data.queries.map((q) => (
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
    </div>
  );
}
