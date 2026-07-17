"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type PostgresConfig } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { AlertTriangle } from "lucide-react";

const FIELDS: {
  key: keyof PostgresConfig;
  label: string;
  hint: string;
  unit: string;
  needsRestart?: boolean;
}[] = [
  {
    key: "max_connections",
    label: "Conexões máximas",
    hint: "Quantas conexões simultâneas o servidor aceita.",
    unit: "",
    needsRestart: true,
  },
  {
    key: "shared_buffers_mb",
    label: "Shared buffers",
    hint: "Memória reservada pro cache interno do Postgres. Regra de bolso: ~25% da RAM do servidor.",
    unit: "MB",
    needsRestart: true,
  },
  {
    key: "work_mem_mb",
    label: "Work mem",
    hint: "Memória por operação de ordenação/hash. Multiplicado por conexões concorrentes — cuidado pra não estourar RAM.",
    unit: "MB",
  },
  {
    key: "maintenance_work_mem_mb",
    label: "Maintenance work mem",
    hint: "Memória usada por VACUUM, CREATE INDEX, etc.",
    unit: "MB",
  },
  {
    key: "effective_cache_size_mb",
    label: "Effective cache size",
    hint: "Estimativa de quanto o SO consegue cachear em disco — ajuda o planner a escolher os planos certos.",
    unit: "MB",
  },
  {
    key: "log_min_duration_statement_ms",
    label: "Log de queries lentas",
    hint: "Loga queries que demorarem mais que isso. -1 desliga.",
    unit: "ms",
  },
];

export function ConfigTab({ serverId, database }: { serverId: string; database: string }) {
  const { data: liveConfig, isLoading } = useQuery({
    queryKey: ["servers", serverId, "config", database],
    queryFn: () => api.getConfig(serverId, database),
    enabled: !!database,
  });

  // Estado local só existe pra guardar edições do usuário — enquanto ele não
  // mexe em nada, os campos mostram direto o que veio do fetch (sem efeito
  // pra "copiar" dado de query pra state, que é anti-padrão no React).
  const [edits, setEdits] = useState<PostgresConfig | null>(null);
  const form: PostgresConfig | null = edits ?? (liveConfig ? stripRestartPending(liveConfig) : null);

  const queryClient = useQueryClient();
  const save = useMutation({
    mutationFn: (cfg: PostgresConfig) => api.updateConfig(serverId, database, cfg),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "config"] });
      setEdits(null);
      if (result.restart_required) {
        toast.warning("Config aplicada, mas alguns parâmetros só valem depois de reiniciar o servidor");
      } else {
        toast.success("Config aplicada");
      }
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao aplicar config"),
  });

  if (isLoading || !form) {
    return <p className="text-muted-foreground p-6 text-sm">Carregando...</p>;
  }

  function setField(key: keyof PostgresConfig, value: number) {
    setEdits({ ...form!, [key]: value });
  }

  return (
    <div className="flex flex-col gap-4">
      {liveConfig?.restart_pending && (
        <div className="flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
          <AlertTriangle className="size-4 shrink-0" />
          Tem mudança pendente de restart pra valer de verdade.
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Configuração do PostgreSQL</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-5">
          {FIELDS.map((field) => (
            <div key={field.key} className="grid gap-1.5">
              <Label htmlFor={field.key} className="flex items-center gap-1.5">
                {field.label}
                {field.needsRestart && (
                  <Badge variant="outline" className="border-amber-200 bg-amber-100 text-amber-700">
                    restart
                  </Badge>
                )}
              </Label>
              <div className="flex items-center gap-2">
                <Input
                  id={field.key}
                  type="number"
                  value={form[field.key]}
                  onChange={(e) => setField(field.key, Number(e.target.value))}
                />
                {field.unit && <span className="text-muted-foreground text-xs">{field.unit}</span>}
              </div>
              <p className="text-muted-foreground text-xs">{field.hint}</p>
            </div>
          ))}
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button onClick={() => save.mutate(form)} disabled={save.isPending}>
          {save.isPending ? "Aplicando..." : "Aplicar configuração"}
        </Button>
      </div>
    </div>
  );
}

function stripRestartPending(cfg: PostgresConfig & { restart_pending?: boolean }): PostgresConfig {
  const { restart_pending: _restartPending, ...rest } = cfg;
  return rest;
}
