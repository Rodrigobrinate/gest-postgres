"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type SlowQuery } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { RotateCcw, Zap, Trash2, Wrench, Copy, ChevronUp, ChevronDown } from "lucide-react";

function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  return `${(bytes / 1024 ** 2).toFixed(0)} MB`;
}

const ORDER_OPTIONS = [
  { value: "total_time", label: "Tempo total" },
  { value: "mean_time", label: "Tempo médio" },
  { value: "calls", label: "Nº de chamadas" },
] as const;

type SortKey = keyof SlowQuery;

const COLUMNS: { key: SortKey; label: string; align: "left" | "right" }[] = [
  { key: "query", label: "Query", align: "left" },
  { key: "calls", label: "Chamadas", align: "right" },
  { key: "total_exec_ms", label: "Tempo total", align: "right" },
  { key: "mean_exec_ms", label: "Tempo médio", align: "right" },
  { key: "rows", label: "Linhas", align: "right" },
  { key: "cache_hit_ratio", label: "Cache hit", align: "right" },
];

function sortQueries(queries: SlowQuery[], key: SortKey, dir: "asc" | "desc") {
  const sorted = [...queries].sort((a, b) => {
    const av = a[key];
    const bv = b[key];
    const cmp = typeof av === "string" && typeof bv === "string" ? av.localeCompare(bv) : Number(av) - Number(bv);
    return dir === "asc" ? cmp : -cmp;
  });
  return sorted;
}

function formatQueryValue(q: SlowQuery, key: SortKey): string {
  switch (key) {
    case "calls":
    case "rows":
      return String(q[key]);
    case "total_exec_ms":
      return `${q.total_exec_ms.toFixed(1)}ms`;
    case "mean_exec_ms":
      return `${q.mean_exec_ms.toFixed(2)}ms`;
    case "cache_hit_ratio":
      return `${(q.cache_hit_ratio * 100).toFixed(0)}%`;
    default:
      return String(q[key]);
  }
}

// copyQueriesToClipboard copia as N primeiras linhas (na ordem atual da
// tabela, já filtrada/ordenada) como TSV — cola direto em planilha.
function copyQueriesToClipboard(queries: SlowQuery[], count: number) {
  const rows = queries.slice(0, count);
  const header = COLUMNS.map((c) => c.label).join("\t");
  const lines = rows.map((q) =>
    COLUMNS.map((c) => (c.key === "query" ? q.query.replace(/[\t\n]/g, " ") : formatQueryValue(q, c.key))).join(
      "\t"
    )
  );
  return navigator.clipboard.writeText([header, ...lines].join("\n"));
}

export function PerformanceTab({ serverId, database }: { serverId: string; database: string }) {
  const [orderBy, setOrderBy] = useState<(typeof ORDER_OPTIONS)[number]["value"]>("total_time");
  const [search, setSearch] = useState("");
  const [minMeanMs, setMinMeanMs] = useState("");
  const [sortKey, setSortKey] = useState<SortKey | null>(null);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [copyOpen, setCopyOpen] = useState(false);
  const [copyCount, setCopyCount] = useState("10");

  function handleHeaderClick(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
  }

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
    mutationFn: () => api.enableQueryStats(serverId, database),
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
  // Sem useMemo de propósito — esse componente já tem um early return
  // condicional acima (fluxo "extensão não habilitada"), então um Hook
  // aqui violaria a regra de Hooks (chamado condicionalmente). Reordenar
  // algumas centenas de linhas a cada render não pesa o bastante pra
  // justificar mover essa lógica pra antes do return só pra usar useMemo.
  const sortedQueries = sortKey ? sortQueries(filteredQueries, sortKey, sortDir) : filteredQueries;

  function confirmCopy() {
    const n = Number(copyCount);
    if (!Number.isFinite(n) || n <= 0) {
      toast.error("Número de linhas inválido");
      return;
    }
    copyQueriesToClipboard(sortedQueries, Math.floor(n)).then(
      () => {
        toast.success(`${Math.min(Math.floor(n), sortedQueries.length)} linha(s) copiada(s)`);
        setCopyOpen(false);
      },
      () => toast.error("Falha ao copiar — navegador bloqueou acesso à área de transferência")
    );
  }

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
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => setCopyOpen(true)}
            disabled={sortedQueries.length === 0}
          >
            <Copy className="size-3.5" />
            Copiar
          </Button>
          <Button size="sm" variant="outline" onClick={() => reset.mutate()} disabled={reset.isPending}>
            <RotateCcw className="size-3.5" />
            Zerar estatísticas
          </Button>
        </div>
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
                    {COLUMNS.map((col) => (
                      <TableHead
                        key={col.key}
                        className={`cursor-pointer select-none ${col.align === "right" ? "text-right" : ""}`}
                        onClick={() => handleHeaderClick(col.key)}
                      >
                        <span className="inline-flex items-center gap-1">
                          {col.label}
                          {sortKey === col.key &&
                            (sortDir === "asc" ? (
                              <ChevronUp className="size-3" />
                            ) : (
                              <ChevronDown className="size-3" />
                            ))}
                        </span>
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sortedQueries.map((q) => (
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

      <Dialog open={copyOpen} onOpenChange={setCopyOpen}>
        <DialogContent className="sm:max-w-xs">
          <DialogHeader>
            <DialogTitle>Copiar linhas</DialogTitle>
          </DialogHeader>
          <div className="grid gap-1.5">
            <Label htmlFor="copy-count">Quantas linhas (de {sortedQueries.length})?</Label>
            <Input
              id="copy-count"
              type="number"
              min={1}
              max={sortedQueries.length}
              value={copyCount}
              onChange={(e) => setCopyCount(e.target.value)}
              autoFocus
            />
            <p className="text-muted-foreground text-xs">
              Copia como texto separado por tab (cola direto em planilha), na ordem atual da tabela.
            </p>
          </div>
          <DialogFooter>
            <Button onClick={confirmCopy}>Copiar</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
