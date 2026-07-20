"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type LogLine } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RefreshCw, ChevronRight, Activity } from "lucide-react";
import { cn } from "@/lib/utils";

const LEVEL_STYLE: Record<string, { badge: string; bar: string; label: string }> = {
  ERROR: { badge: "bg-red-500/15 text-red-400 border-red-500/30", bar: "bg-red-500", label: "error" },
  FATAL: { badge: "bg-red-500/15 text-red-400 border-red-500/30", bar: "bg-red-500", label: "fatal" },
  PANIC: { badge: "bg-red-500/15 text-red-400 border-red-500/30", bar: "bg-red-500", label: "panic" },
  WARNING: { badge: "bg-amber-500/15 text-amber-400 border-amber-500/30", bar: "bg-amber-500", label: "warning" },
  NOTICE: { badge: "bg-blue-500/15 text-blue-400 border-blue-500/30", bar: "bg-blue-500", label: "notice" },
  INFO: { badge: "bg-blue-500/15 text-blue-400 border-blue-500/30", bar: "bg-blue-500", label: "info" },
  LOG: { badge: "bg-zinc-500/15 text-zinc-300 border-zinc-500/30", bar: "bg-zinc-500", label: "log" },
  DEBUG1: { badge: "bg-purple-500/15 text-purple-400 border-purple-500/30", bar: "bg-purple-500", label: "debug" },
  DEBUG2: { badge: "bg-purple-500/15 text-purple-400 border-purple-500/30", bar: "bg-purple-500", label: "debug" },
  DEBUG3: { badge: "bg-purple-500/15 text-purple-400 border-purple-500/30", bar: "bg-purple-500", label: "debug" },
  DEBUG4: { badge: "bg-purple-500/15 text-purple-400 border-purple-500/30", bar: "bg-purple-500", label: "debug" },
  DEBUG5: { badge: "bg-purple-500/15 text-purple-400 border-purple-500/30", bar: "bg-purple-500", label: "debug" },
};

function levelStyle(level: string) {
  return LEVEL_STYLE[level] ?? { badge: "bg-zinc-700/30 text-zinc-400 border-zinc-600/30", bar: "bg-zinc-700", label: level ? level.toLowerCase() : "—" };
}

function rowKey(l: LogLine, i: number) {
  return `${l.timestamp}-${i}-${l.text}`;
}

export function LogsTab({ serverId }: { serverId: string }) {
  const [search, setSearch] = useState("");
  const [levelFilter, setLevelFilter] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [showMetrics, setShowMetrics] = useState(false);

  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["servers", serverId, "logs-timeline"],
    queryFn: () => api.logsTimeline(serverId, 500),
    refetchInterval: 5000,
  });

  const levels = useMemo(() => {
    const set = new Set<string>();
    for (const l of data ?? []) set.add(l.level || "—");
    return Array.from(set).sort();
  }, [data]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (data ?? []).filter((l) => {
      if (levelFilter && (l.level || "—") !== levelFilter) return false;
      if (!q) return true;
      if (l.text.toLowerCase().includes(q)) return true;
      return (l.details ?? []).some((d) => d.toLowerCase().includes(q));
    });
  }, [data, search, levelFilter]);

  function toggle(key: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  return (
    <Card>
      <CardContent className="p-4">
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <Input
            placeholder="Filtrar por texto..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-8 max-w-xs text-xs"
          />
          <div className="flex flex-wrap gap-1.5">
            <button
              onClick={() => setLevelFilter(null)}
              className={cn(
                "rounded-full border px-2.5 py-0.5 text-xs",
                levelFilter === null ? "border-foreground bg-foreground text-background" : "border-border text-muted-foreground"
              )}
            >
              Todos
            </button>
            {levels.map((lv) => {
              const style = levelStyle(lv === "—" ? "" : lv);
              return (
                <button
                  key={lv}
                  onClick={() => setLevelFilter((cur) => (cur === lv ? null : lv))}
                  className={cn(
                    "rounded-full border px-2.5 py-0.5 text-xs capitalize",
                    levelFilter === lv ? style.badge.replace("/15", "/25") : style.badge
                  )}
                >
                  {style.label}
                </button>
              );
            })}
          </div>
          <div className="ml-auto flex items-center gap-2">
            <Button size="sm" variant={showMetrics ? "default" : "outline"} onClick={() => setShowMetrics((v) => !v)}>
              <Activity className="size-3.5" />
              CPU/conexões
            </Button>
            <Button size="sm" variant="outline" onClick={() => refetch()} disabled={isFetching}>
              <RefreshCw className={cn("size-3.5", isFetching && "animate-spin")} />
              Atualizar
            </Button>
          </div>
        </div>

        <div className="max-h-[560px] overflow-auto rounded-md border bg-zinc-950 text-xs text-zinc-100">
          {isLoading ? (
            <p className="p-4 text-zinc-400">Carregando...</p>
          ) : filtered.length === 0 ? (
            <p className="p-4 text-zinc-400">{data && data.length > 0 ? "Nenhuma linha bate com o filtro." : "Sem logs ainda."}</p>
          ) : (
            <table className="w-full border-collapse">
              <tbody>
                {filtered.map((l, i) => {
                  const key = rowKey(l, i);
                  const style = levelStyle(l.level);
                  const isOpen = expanded.has(key);
                  const hasMore = (l.details ?? []).length > 0;
                  return (
                    <FragmentRow
                      key={key}
                      line={l}
                      style={style}
                      isOpen={isOpen}
                      hasMore={hasMore}
                      showMetrics={showMetrics}
                      onToggle={() => hasMore && toggle(key)}
                    />
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
        <p className="text-muted-foreground mt-2 text-xs">
          Últimas {data?.length ?? 0} linhas · atualiza a cada 5s
        </p>
      </CardContent>
    </Card>
  );
}

function FragmentRow({
  line,
  style,
  isOpen,
  hasMore,
  showMetrics,
  onToggle,
}: {
  line: LogLine;
  style: { badge: string; bar: string; label: string };
  isOpen: boolean;
  hasMore: boolean;
  showMetrics: boolean;
  onToggle: () => void;
}) {
  return (
    <>
      <tr
        className={cn("border-b border-zinc-900 hover:bg-zinc-900/60", hasMore && "cursor-pointer")}
        onClick={onToggle}
      >
        <td className="w-6 py-1.5 pl-2 align-top">
          {hasMore && (
            <ChevronRight className={cn("size-3.5 text-zinc-500 transition-transform", isOpen && "rotate-90")} />
          )}
        </td>
        <td className={cn("w-1 p-0", style.bar)} />
        <td className="text-muted-foreground w-40 shrink-0 px-3 py-1.5 align-top whitespace-nowrap">
          {line.timestamp ? new Date(line.timestamp).toLocaleString("pt-BR") : "—"}
        </td>
        <td className="w-24 shrink-0 px-1 py-1.5 align-top">
          <span className={cn("inline-block rounded-full border px-2 py-0.5 text-[10px] capitalize", style.badge)}>
            {style.label}
          </span>
        </td>
        {showMetrics && (
          <td className="w-32 shrink-0 px-3 py-1.5 align-top whitespace-nowrap">
            {line.cpu_percent != null && (
              <span className={cn(line.cpu_percent >= 70 ? "text-red-400" : line.cpu_percent >= 30 ? "text-amber-400" : "text-zinc-500")}>
                CPU {line.cpu_percent.toFixed(0)}%
              </span>
            )}
            {line.connection_count != null && <span className="text-zinc-500"> · {line.connection_count} conn</span>}
          </td>
        )}
        <td className="px-3 py-1.5 font-mono whitespace-pre-wrap">{line.text}</td>
      </tr>
      {isOpen && hasMore && (
        <tr className="border-b border-zinc-900 bg-zinc-900/40">
          <td />
          <td className={cn("w-1 p-0", style.bar)} />
          <td colSpan={showMetrics ? 4 : 3} className="px-3 py-2 font-mono whitespace-pre-wrap text-zinc-400">
            {(line.details ?? []).join("\n")}
          </td>
        </tr>
      )}
    </>
  );
}
