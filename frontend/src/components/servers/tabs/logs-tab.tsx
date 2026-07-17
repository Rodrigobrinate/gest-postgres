"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { RefreshCw } from "lucide-react";
import { cn } from "@/lib/utils";

export function LogsTab({ serverId }: { serverId: string }) {
  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["servers", serverId, "logs"],
    queryFn: () => api.logs(serverId, 500),
    refetchInterval: 5000,
  });

  return (
    <Card>
      <CardContent className="p-4">
        <div className="mb-2 flex items-center justify-between">
          <p className="text-muted-foreground text-xs">Últimas 500 linhas de stdout/stderr do container</p>
          <Button size="sm" variant="outline" onClick={() => refetch()} disabled={isFetching}>
            <RefreshCw className={cn("size-3.5", isFetching && "animate-spin")} />
            Atualizar
          </Button>
        </div>
        <pre className="max-h-[500px] overflow-auto rounded-md bg-zinc-950 p-4 text-xs whitespace-pre-wrap text-zinc-100">
          {isLoading ? "Carregando..." : data?.logs || "Sem logs ainda."}
        </pre>
      </CardContent>
    </Card>
  );
}
