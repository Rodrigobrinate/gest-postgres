"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { AlertTriangle, Search, Wand2 } from "lucide-react";

export function ConfigTab({ serverId, database }: { serverId: string; database: string }) {
  const { data: params, isLoading } = useQuery({
    queryKey: ["servers", serverId, "config", database],
    queryFn: () => api.getConfig(serverId, database),
    enabled: !!database,
  });

  const { data: tuning } = useQuery({
    queryKey: ["servers", serverId, "tuning-suggestions"],
    queryFn: () => api.tuningSuggestions(serverId),
  });

  const [showTuning, setShowTuning] = useState(false);
  const [filter, setFilter] = useState("");
  const [edits, setEdits] = useState<Record<string, string>>({});

  const queryClient = useQueryClient();
  const save = useMutation({
    mutationFn: () => api.updateConfig(serverId, database, edits),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "config"] });
      setEdits({});
      if (result.restart_required) {
        toast.warning("Config aplicada — alguns parâmetros só valem depois de reiniciar o servidor");
      } else {
        toast.success("Config aplicada");
      }
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao aplicar config"),
  });

  const grouped = useMemo(() => {
    if (!params) return [];
    const q = filter.trim().toLowerCase();
    const filtered = q
      ? params.filter(
          (p) =>
            p.label.toLowerCase().includes(q) ||
            p.name.toLowerCase().includes(q) ||
            p.hint.toLowerCase().includes(q)
        )
      : params;

    const byCategory = new Map<string, typeof filtered>();
    for (const p of filtered) {
      const list = byCategory.get(p.category) ?? [];
      list.push(p);
      byCategory.set(p.category, list);
    }
    return Array.from(byCategory.entries());
  }, [params, filter]);

  if (isLoading || !params) {
    return <p className="text-muted-foreground p-6 text-sm">Carregando...</p>;
  }

  const pendingRestart = params.some((p) => p.pending_restart);
  const editCount = Object.keys(edits).length;
  const tuningDiffs = tuning?.filter((t) => t.differs) ?? [];

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardContent className="p-4">
          <button
            className="flex w-full items-center justify-between text-left"
            onClick={() => setShowTuning((v) => !v)}
          >
            <span className="flex items-center gap-2 text-sm font-semibold">
              <Wand2 className="size-4" />
              Tuning assistido (baseado nos recursos do container)
              {tuningDiffs.length > 0 && (
                <Badge variant="outline">{tuningDiffs.length} sugestão(ões)</Badge>
              )}
            </span>
            <span className="text-muted-foreground text-xs">{showTuning ? "ocultar" : "ver"}</span>
          </button>
          {showTuning && (
            <div className="mt-3 flex flex-col gap-2">
              {!tuning ? (
                <p className="text-muted-foreground text-sm">Carregando...</p>
              ) : (
                tuning.map((t) => (
                  <div
                    key={t.param}
                    className="flex flex-wrap items-center justify-between gap-2 border-t pt-2 text-sm first:border-t-0 first:pt-0"
                  >
                    <div className="min-w-0">
                      <span className="font-mono text-xs font-medium">{t.param}</span>
                      <p className="text-muted-foreground text-xs">
                        atual: <span className="font-mono">{t.current_value}</span> · sugerido:{" "}
                        <span className="font-mono">{t.suggested_value}</span>
                      </p>
                      <p className="text-muted-foreground text-xs">{t.reason}</p>
                    </div>
                    {t.differs && (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() =>
                          setEdits((cur) => ({ ...cur, [t.param]: t.suggested_value }))
                        }
                      >
                        Usar sugestão
                      </Button>
                    )}
                  </div>
                ))
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {pendingRestart && (
        <div className="flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
          <AlertTriangle className="size-4 shrink-0" />
          Tem mudança pendente de restart pra valer de verdade.
        </div>
      )}

      <div className="flex items-center justify-between gap-3">
        <div className="relative max-w-sm flex-1">
          <Search className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
          <Input
            placeholder="Buscar parâmetro... (ex: work_mem, timeout, log)"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="pl-8"
          />
        </div>
        <Button onClick={() => save.mutate()} disabled={save.isPending || editCount === 0}>
          {save.isPending ? "Aplicando..." : editCount > 0 ? `Aplicar (${editCount})` : "Aplicar"}
        </Button>
      </div>

      {grouped.length === 0 ? (
        <p className="text-muted-foreground p-6 text-center text-sm">Nada encontrado.</p>
      ) : (
        grouped.map(([category, items]) => (
          <Card key={category}>
            <CardContent className="p-4">
              <h3 className="mb-3 text-sm font-semibold">{category}</h3>
              <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                {items.map((p) => (
                  <div key={p.name} className="grid gap-1.5">
                    <Label htmlFor={p.name} className="flex items-center gap-1.5 text-xs">
                      {p.label}
                      <span className="text-muted-foreground font-mono">({p.name})</span>
                      {p.restart && (
                        <Badge
                          variant="outline"
                          className="border-amber-200 bg-amber-100 text-[10px] text-amber-700"
                        >
                          restart
                        </Badge>
                      )}
                    </Label>
                    <Input
                      id={p.name}
                      value={edits[p.name] ?? p.value}
                      onChange={(e) => setEdits((cur) => ({ ...cur, [p.name]: e.target.value }))}
                      className="h-8 font-mono text-xs"
                    />
                    <p className="text-muted-foreground text-xs">{p.hint}</p>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        ))
      )}
    </div>
  );
}
