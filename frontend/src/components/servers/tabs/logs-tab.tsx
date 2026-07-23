"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type LogLine } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { RefreshCw, ChevronRight, Activity, Pause, Play, Download, ArrowDown } from "lucide-react";
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

const TAIL_OPTIONS = [200, 500, 1000, 2000];

// Distância (px) até o fundo do painel de log ainda contada como "colado no
// fim" — abaixo disso, novo dado chegando faz o painel acompanhar sozinho
// (tail -f); acima disso, o usuário tá lendo linha antiga de propósito, não
// pode ser puxado pra baixo à força a cada refetch de 5s.
const STICK_TO_BOTTOM_THRESHOLD_PX = 48;

function levelStyle(level: string) {
  return (
    LEVEL_STYLE[level] ?? {
      badge: "bg-zinc-700/30 text-zinc-400 border-zinc-600/30",
      bar: "bg-zinc-700",
      label: level ? level.toLowerCase() : "—",
    }
  );
}

function rowKey(l: LogLine, i: number) {
  return `${l.timestamp}-${i}-${l.text}`;
}

// highlightMatch — grifa o trecho que bateu na busca de texto livre, pra
// achar o "porquê" da linha ter passado no filtro sem precisar reler tudo.
function highlightMatch(text: string, query: string) {
  if (!query) return text;
  const idx = text.toLowerCase().indexOf(query.toLowerCase());
  if (idx === -1) return text;
  return (
    <>
      {text.slice(0, idx)}
      <mark className="rounded-sm bg-yellow-500/40 text-inherit">{text.slice(idx, idx + query.length)}</mark>
      {text.slice(idx + query.length)}
    </>
  );
}

export function LogsTab({ serverId }: { serverId: string }) {
  const [search, setSearch] = useState("");
  const [levelFilters, setLevelFilters] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [showMetrics, setShowMetrics] = useState(false);
  const [live, setLive] = useState(true);
  const [tailLines, setTailLines] = useState(500);
  const [newSinceView, setNewSinceView] = useState(0);

  const scrollRef = useRef<HTMLDivElement>(null);
  const stickToBottomRef = useRef(true);
  const seenKeysRef = useRef<Set<string>>(new Set());
  const firstLoadRef = useRef(true);

  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["servers", serverId, "logs-timeline", tailLines],
    queryFn: () => api.logsTimeline(serverId, tailLines),
    refetchInterval: live ? 5000 : false,
  });

  const levels = useMemo(() => {
    const set = new Set<string>();
    for (const l of data ?? []) set.add(l.level || "—");
    return Array.from(set).sort();
  }, [data]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (data ?? []).filter((l) => {
      if (levelFilters.size > 0 && !levelFilters.has(l.level || "—")) return false;
      if (!q) return true;
      if (l.text.toLowerCase().includes(q)) return true;
      return (l.details ?? []).some((d) => d.toLowerCase().includes(q));
    });
  }, [data, search, levelFilters]);

  // Conta quanto do que chegou é realmente novo (chave nunca vista) — vira o
  // "N novas linhas" do botão flutuante quando o usuário rolou pra cima.
  useEffect(() => {
    if (!data) return;
    const seen = seenKeysRef.current;
    let added = 0;
    const nextSeen = new Set<string>();
    data.forEach((l, i) => {
      const key = rowKey(l, i);
      nextSeen.add(key);
      if (!seen.has(key)) added++;
    });
    seenKeysRef.current = nextSeen;

    if (firstLoadRef.current) {
      firstLoadRef.current = false;
      // primeira carga: começa colado no fim, igual `tail -f`
      requestAnimationFrame(() => {
        const el = scrollRef.current;
        if (el) el.scrollTop = el.scrollHeight;
      });
      return;
    }

    if (stickToBottomRef.current) {
      requestAnimationFrame(() => {
        const el = scrollRef.current;
        if (el) el.scrollTop = el.scrollHeight;
      });
    } else if (added > 0) {
      // Deferido pro próximo frame (não direto no corpo do efeito) — mesmo
      // motivo de embrulhar o scrollTop acima em requestAnimationFrame.
      requestAnimationFrame(() => setNewSinceView((n) => n + added));
    }
  }, [data]);

  function handleScroll() {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < STICK_TO_BOTTOM_THRESHOLD_PX;
    stickToBottomRef.current = atBottom;
    if (atBottom) setNewSinceView(0);
  }

  function jumpToBottom() {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
    stickToBottomRef.current = true;
    setNewSinceView(0);
  }

  function toggleLevel(lv: string) {
    setLevelFilters((prev) => {
      const next = new Set(prev);
      if (next.has(lv)) next.delete(lv);
      else next.add(lv);
      return next;
    });
  }

  function toggle(key: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function downloadLogs() {
    const lines = filtered.map((l) => {
      const detail = (l.details ?? []).map((d) => `    ${d}`).join("\n");
      return detail ? `${l.text}\n${detail}` : l.text;
    });
    const blob = new Blob([lines.join("\n")], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `logs-${serverId}-${new Date().toISOString().replace(/[:.]/g, "-")}.log`;
    a.click();
    URL.revokeObjectURL(url);
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
              onClick={() => setLevelFilters(new Set())}
              className={cn(
                "rounded-full border px-2.5 py-0.5 text-xs",
                levelFilters.size === 0
                  ? "border-foreground bg-foreground text-background"
                  : "border-border text-muted-foreground"
              )}
            >
              Todos
            </button>
            {levels.map((lv) => {
              const style = levelStyle(lv === "—" ? "" : lv);
              const active = levelFilters.has(lv);
              return (
                <button
                  key={lv}
                  onClick={() => toggleLevel(lv)}
                  className={cn(
                    "rounded-full border px-2.5 py-0.5 text-xs capitalize",
                    active ? style.badge.replace("/15", "/25") : style.badge
                  )}
                >
                  {style.label}
                </button>
              );
            })}
          </div>
          <div className="ml-auto flex items-center gap-2">
            <Select value={String(tailLines)} onValueChange={(v) => v && setTailLines(Number(v))}>
              <SelectTrigger className="h-8 w-28 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {TAIL_OPTIONS.map((n) => (
                  <SelectItem key={n} value={String(n)}>
                    {n} linhas
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button size="sm" variant={showMetrics ? "default" : "outline"} onClick={() => setShowMetrics((v) => !v)}>
              <Activity className="size-3.5" />
              CPU/conexões
            </Button>
            <Button size="sm" variant={live ? "default" : "outline"} onClick={() => setLive((v) => !v)}>
              {live ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
              {live ? "Ao vivo" : "Pausado"}
            </Button>
            <Button size="sm" variant="outline" onClick={() => refetch()} disabled={isFetching}>
              <RefreshCw className={cn("size-3.5", isFetching && "animate-spin")} />
              Atualizar
            </Button>
            <Button size="sm" variant="outline" onClick={downloadLogs} disabled={filtered.length === 0}>
              <Download className="size-3.5" />
              Baixar
            </Button>
          </div>
        </div>

        <div className="relative">
          <div
            ref={scrollRef}
            onScroll={handleScroll}
            className="max-h-[560px] overflow-auto rounded-md border bg-zinc-950 text-xs text-zinc-100"
          >
            {isLoading ? (
              <p className="p-4 text-zinc-400">Carregando...</p>
            ) : filtered.length === 0 ? (
              <p className="p-4 text-zinc-400">
                {data && data.length > 0 ? "Nenhuma linha bate com o filtro." : "Sem logs ainda."}
              </p>
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
                        search={search.trim()}
                        onToggle={() => hasMore && toggle(key)}
                      />
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>

          {newSinceView > 0 && (
            <button
              onClick={jumpToBottom}
              className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-foreground px-3 py-1.5 text-xs font-medium text-background shadow-lg"
            >
              <ArrowDown className="size-3.5" />
              {newSinceView} nova(s) linha(s)
            </button>
          )}
        </div>

        <p className="text-muted-foreground mt-2 text-xs">
          {filtered.length} de {data?.length ?? 0} linha(s)
          {(search.trim() || levelFilters.size > 0) && " (filtradas)"} · {live ? "atualiza a cada 5s" : "pausado"}
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
  search,
  onToggle,
}: {
  line: LogLine;
  style: { badge: string; bar: string; label: string };
  isOpen: boolean;
  hasMore: boolean;
  showMetrics: boolean;
  search: string;
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
              <span
                className={cn(
                  line.cpu_percent >= 70 ? "text-red-400" : line.cpu_percent >= 30 ? "text-amber-400" : "text-zinc-500"
                )}
              >
                CPU {line.cpu_percent.toFixed(0)}%
              </span>
            )}
            {line.connection_count != null && <span className="text-zinc-500"> · {line.connection_count} conn</span>}
          </td>
        )}
        <td className="px-3 py-1.5 font-mono whitespace-pre-wrap">{highlightMatch(line.text, search)}</td>
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
