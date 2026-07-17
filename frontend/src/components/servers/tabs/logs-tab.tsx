"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { RefreshCw, Activity } from "lucide-react";
import { cn } from "@/lib/utils";

export function LogsTab({ serverId }: { serverId: string }) {
  const [correlate, setCorrelate] = useState(false);

  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["servers", serverId, "logs"],
    queryFn: () => api.logs(serverId, 500),
    refetchInterval: correlate ? false : 5000,
    enabled: !correlate,
  });

  const {
    data: timeline,
    isLoading: timelineLoading,
    refetch: refetchTimeline,
    isFetching: timelineFetching,
  } = useQuery({
    queryKey: ["servers", serverId, "logs-timeline"],
    queryFn: () => api.logsTimeline(serverId, 300),
    refetchInterval: correlate ? 15000 : false,
    enabled: correlate,
  });

  return (
    <Card>
      <CardContent className="p-4">
        <div className="mb-2 flex items-center justify-between">
          <p className="text-muted-foreground text-xs">
            {correlate
              ? "Últimas 300 linhas, anotadas com CPU/conexões do momento (histórico em memória)"
              : "Últimas 500 linhas de stdout/stderr do container"}
          </p>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant={correlate ? "default" : "outline"}
              onClick={() => setCorrelate((v) => !v)}
            >
              <Activity className="size-3.5" />
              Correlacionar com métricas
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => (correlate ? refetchTimeline() : refetch())}
              disabled={correlate ? timelineFetching : isFetching}
            >
              <RefreshCw className={cn("size-3.5", (correlate ? timelineFetching : isFetching) && "animate-spin")} />
              Atualizar
            </Button>
          </div>
        </div>

        {!correlate ? (
          <pre className="max-h-[500px] overflow-auto rounded-md bg-zinc-950 p-4 text-xs whitespace-pre-wrap text-zinc-100">
            {isLoading ? "Carregando..." : data?.logs || "Sem logs ainda."}
          </pre>
        ) : (
          <div className="max-h-[500px] overflow-auto rounded-md bg-zinc-950 text-xs text-zinc-100">
            {timelineLoading ? (
              <p className="p-4">Carregando...</p>
            ) : !timeline || timeline.length === 0 ? (
              <p className="p-4">Sem logs ainda.</p>
            ) : (
              <table className="w-full border-collapse">
                <tbody>
                  {timeline.map((l, i) => (
                    <tr key={i} className="hover:bg-zinc-900">
                      <td className="text-muted-foreground w-32 shrink-0 px-3 py-0.5 align-top whitespace-nowrap">
                        {l.timestamp ? new Date(l.timestamp).toLocaleTimeString("pt-BR") : "—"}
                      </td>
                      <td className="w-32 shrink-0 px-3 py-0.5 align-top whitespace-nowrap">
                        {l.cpu_percent != null && (
                          <span
                            className={cn(
                              l.cpu_percent >= 70
                                ? "text-red-400"
                                : l.cpu_percent >= 30
                                  ? "text-amber-400"
                                  : "text-zinc-500"
                            )}
                          >
                            CPU {l.cpu_percent.toFixed(0)}%
                          </span>
                        )}
                        {l.connection_count != null && (
                          <span className="text-zinc-500"> · {l.connection_count} conn</span>
                        )}
                      </td>
                      <td className="px-3 py-0.5 whitespace-pre-wrap">{l.text}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
