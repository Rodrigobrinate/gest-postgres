"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Network } from "lucide-react";

const POOL_MODES = [
  { value: "transaction", label: "Transaction (recomendado)" },
  { value: "session", label: "Session" },
  { value: "statement", label: "Statement" },
];

export function ConnectionPooling({ serverId }: { serverId: string }) {
  const { data: server } = useQuery({
    queryKey: ["servers", serverId],
    queryFn: () => api.getServer(serverId),
  });
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId] });
  const [poolMode, setPoolMode] = useState("transaction");

  const enable = useMutation({
    mutationFn: () => api.enablePooling(serverId, poolMode),
    onSuccess: () => {
      toast.success("Pooling habilitado");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao habilitar pooling"),
  });

  const disable = useMutation({
    mutationFn: () => api.disablePooling(serverId),
    onSuccess: () => {
      toast.success("Pooling desabilitado");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao desabilitar pooling"),
  });

  if (!server) return null;

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="flex items-center gap-1.5 text-sm font-medium">
            <Network className="size-4" />
            Connection pooling (PgBouncer)
            {server.pooler_enabled && <Badge variant="outline">ativo</Badge>}
          </span>
          {server.pooler_enabled ? (
            <Button size="sm" variant="outline" disabled={disable.isPending} onClick={() => disable.mutate()}>
              {disable.isPending ? "Desabilitando..." : "Desabilitar"}
            </Button>
          ) : (
            <div className="flex items-center gap-2">
              <Select value={poolMode} onValueChange={(v) => v && setPoolMode(v)}>
                <SelectTrigger className="h-8 w-52 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {POOL_MODES.map((m) => (
                    <SelectItem key={m.value} value={m.value}>
                      {m.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button size="sm" disabled={enable.isPending} onClick={() => enable.mutate()}>
                {enable.isPending ? "Habilitando..." : "Habilitar"}
              </Button>
            </div>
          )}
        </div>
        {server.pooler_enabled ? (
          <p className="text-muted-foreground mt-2 text-xs">
            Conecte na porta <span className="font-mono">{server.pooler_host_port}</span> em vez da{" "}
            <span className="font-mono">{server.host_port}</span> — mesmo usuário/senha/banco, modo{" "}
            <span className="font-mono">{server.pooler_pool_mode}</span>. Container pgbouncer próprio, sem
            tocar no Postgres.
          </p>
        ) : (
          <p className="text-muted-foreground mt-2 text-xs">
            Sobe um pgbouncer dedicado apontando pra esse Postgres, numa porta própria — útil quando muitas
            conexões curtas (ex: serverless) esgotariam max_connections direto no banco.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
