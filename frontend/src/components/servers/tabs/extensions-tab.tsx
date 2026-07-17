"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Search } from "lucide-react";

// Extensões mais pedidas ficam no topo quando ainda não habilitadas, pra não
// precisar catar entre as ~40 que o Postgres lista por padrão.
const HIGHLIGHTED = ["pg_stat_statements", "uuid-ossp", "pgcrypto", "pg_trgm", "hstore", "citext"];

export function ExtensionsTab({ serverId, database }: { serverId: string; database: string }) {
  const [filter, setFilter] = useState("");

  const { data: extensions, isLoading } = useQuery({
    queryKey: ["servers", serverId, "extensions", database],
    queryFn: () => api.listExtensions(serverId, database),
    enabled: !!database,
  });

  const queryClient = useQueryClient();
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "extensions"] });

  const enable = useMutation({
    mutationFn: (name: string) => api.enableExtension(serverId, database, name),
    onSuccess: (_data, name) => {
      toast.success(`${name} habilitada`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao habilitar"),
  });

  const disable = useMutation({
    mutationFn: (name: string) => api.disableExtension(serverId, database, name),
    onSuccess: (_data, name) => {
      toast.success(`${name} desabilitada`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao desabilitar"),
  });

  const filtered = (extensions ?? []).filter(
    (e) =>
      e.name.toLowerCase().includes(filter.toLowerCase()) ||
      e.comment.toLowerCase().includes(filter.toLowerCase())
  );

  const sorted = [...filtered].sort((a, b) => {
    const installedDiff = Number(!!b.installed_version) - Number(!!a.installed_version);
    if (installedDiff !== 0) return installedDiff;
    const highlightDiff = Number(HIGHLIGHTED.includes(b.name)) - Number(HIGHLIGHTED.includes(a.name));
    if (highlightDiff !== 0) return highlightDiff;
    return a.name.localeCompare(b.name);
  });

  return (
    <div className="flex flex-col gap-4">
      <div className="relative max-w-sm">
        <Search className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
        <Input
          placeholder="Buscar extensão..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="pl-8"
        />
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : sorted.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhuma extensão encontrada.</p>
          ) : (
            <ul className="divide-y">
              {sorted.map((ext) => {
                const installed = !!ext.installed_version;
                const busy =
                  (enable.isPending && enable.variables === ext.name) ||
                  (disable.isPending && disable.variables === ext.name);
                return (
                  <li key={ext.name} className="flex items-center justify-between gap-4 p-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm font-medium">{ext.name}</span>
                        {installed && (
                          <Badge variant="outline" className="border-emerald-200 bg-emerald-100 text-emerald-700">
                            v{ext.installed_version}
                          </Badge>
                        )}
                      </div>
                      {ext.comment && (
                        <p className="text-muted-foreground truncate text-xs">{ext.comment}</p>
                      )}
                    </div>
                    <Button
                      size="sm"
                      variant={installed ? "outline" : "default"}
                      disabled={busy}
                      onClick={() =>
                        installed ? disable.mutate(ext.name) : enable.mutate(ext.name)
                      }
                    >
                      {busy ? "..." : installed ? "Desabilitar" : "Habilitar"}
                    </Button>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
