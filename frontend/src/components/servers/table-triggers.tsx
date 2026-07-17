"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { CreateTriggerDialog } from "./create-trigger-dialog";
import { Trash2, Zap, ZapOff } from "lucide-react";

export function TableTriggers({
  serverId,
  database,
  schema,
  table,
}: {
  serverId: string;
  database: string;
  schema: string;
  table: string;
}) {
  const { data: triggers, isLoading } = useQuery({
    queryKey: ["servers", serverId, "triggers", database, schema, table],
    queryFn: () => api.listTriggers(serverId, database, schema, table),
  });

  const queryClient = useQueryClient();
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "triggers"] });

  const toggle = useMutation({
    mutationFn: ({ name, enabled }: { name: string; enabled: boolean }) =>
      api.setTriggerEnabled(serverId, database, schema, table, name, enabled),
    onSuccess: (_d, { name, enabled }) => {
      toast.success(`${name} ${enabled ? "habilitado" : "desabilitado"}`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar trigger"),
  });

  const remove = useMutation({
    mutationFn: (name: string) => api.dropTrigger(serverId, database, schema, table, name),
    onSuccess: (_d, name) => {
      toast.success(`${name} excluído`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir trigger"),
  });

  return (
    <div className="flex flex-col gap-2 border-t p-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">Triggers</span>
        <CreateTriggerDialog serverId={serverId} database={database} schema={schema} table={table} />
      </div>

      {isLoading ? (
        <p className="text-muted-foreground text-xs">Carregando...</p>
      ) : !triggers || triggers.length === 0 ? (
        <p className="text-muted-foreground text-xs">Nenhum trigger nessa tabela.</p>
      ) : (
        <ul className="divide-y">
          {triggers.map((t) => (
            <li key={t.name} className="flex items-center justify-between gap-3 py-2">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="font-mono text-sm">{t.name}</span>
                  <Badge
                    variant="outline"
                    className={
                      t.enabled
                        ? "border-emerald-200 bg-emerald-100 text-emerald-700"
                        : "border-zinc-200 bg-zinc-100 text-zinc-600"
                    }
                  >
                    {t.enabled ? "ativo" : "desativado"}
                  </Badge>
                </div>
                <p className="text-muted-foreground truncate font-mono text-xs">{t.definition}</p>
              </div>
              <div className="flex shrink-0 gap-1">
                <Button
                  size="icon"
                  variant="ghost"
                  title={t.enabled ? "Desabilitar" : "Habilitar"}
                  onClick={() => toggle.mutate({ name: t.name, enabled: !t.enabled })}
                >
                  {t.enabled ? <ZapOff className="size-4" /> : <Zap className="size-4" />}
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  title="Excluir"
                  className="text-red-600 hover:text-red-700"
                  onClick={() => remove.mutate(t.name)}
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
