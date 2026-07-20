"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type InfraContainer } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { CreateContainerDialog } from "@/components/infra/create-container-dialog";
import { ContainerLogsPane } from "@/components/infra/container-logs-pane";
import { Box, FileText, Play, RotateCw, Square, Trash2 } from "lucide-react";
import { cn } from "@/lib/utils";

export function ContainersTab() {
  const { data: containers, isLoading } = useQuery({
    queryKey: ["infra-containers"],
    queryFn: () => api.listInfraContainers(),
    refetchInterval: 10_000,
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["infra-containers"] });

  const action = useMutation({
    mutationFn: async (args: { id: string; op: "start" | "stop" | "restart" | "remove" }) => {
      if (args.op === "start") await api.startInfraContainer(args.id);
      else if (args.op === "stop") await api.stopInfraContainer(args.id);
      else if (args.op === "restart") await api.restartInfraContainer(args.id);
      else await api.removeInfraContainer(args.id);
    },
    onSuccess: invalidate,
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha na ação"),
  });

  const [logsFor, setLogsFor] = useState<InfraContainer | null>(null);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <Box className="size-4" />
          Containers
        </CardTitle>
        <CreateContainerDialog onCreated={invalidate} />
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !containers || containers.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhum container.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-muted-foreground border-b text-xs">
                  <th className="px-4 py-2 text-left font-normal">Nome</th>
                  <th className="px-4 py-2 text-left font-normal">Imagem</th>
                  <th className="px-4 py-2 text-left font-normal">Estado</th>
                  <th className="px-4 py-2 text-left font-normal">Redes</th>
                  <th className="px-4 py-2 text-left font-normal">Portas</th>
                  <th className="px-4 py-2 text-right font-normal">Ações</th>
                </tr>
              </thead>
              <tbody>
                {containers.map((c) => (
                  <tr key={c.id} className="border-b last:border-0">
                    <td className="px-4 py-2">
                      <div className="flex items-center gap-2">
                        <Link
                          href={`/infra/containers/${c.id}`}
                          className="truncate font-mono hover:underline"
                        >
                          {c.name}
                        </Link>
                        {c.project && <Badge variant="outline">{c.project}</Badge>}
                      </div>
                    </td>
                    <td className="text-muted-foreground px-4 py-2 font-mono text-xs">{c.image}</td>
                    <td className="px-4 py-2">
                      <span
                        className={cn(
                          "text-xs font-medium",
                          c.state === "running" ? "text-emerald-600" : "text-muted-foreground"
                        )}
                      >
                        {c.state}
                      </span>
                    </td>
                    <td className="text-muted-foreground px-4 py-2 font-mono text-xs">
                      {c.networks.join(", ") || "—"}
                    </td>
                    <td className="text-muted-foreground px-4 py-2 font-mono text-xs">
                      {c.ports.length > 0 ? c.ports.join(", ") : "—"}
                    </td>
                    <td className="px-4 py-2">
                      <div className="flex justify-end gap-1">
                        <Button size="icon" variant="ghost" title="Logs" onClick={() => setLogsFor(c)}>
                          <FileText className="size-4" />
                        </Button>
                        {c.state === "running" ? (
                          <>
                            <Button
                              size="icon"
                              variant="ghost"
                              title="Parar"
                              disabled={action.isPending}
                              onClick={() => action.mutate({ id: c.id, op: "stop" })}
                            >
                              <Square className="size-4" />
                            </Button>
                            <Button
                              size="icon"
                              variant="ghost"
                              title="Reiniciar"
                              disabled={action.isPending}
                              onClick={() => action.mutate({ id: c.id, op: "restart" })}
                            >
                              <RotateCw className="size-4" />
                            </Button>
                          </>
                        ) : (
                          <Button
                            size="icon"
                            variant="ghost"
                            title="Iniciar"
                            disabled={action.isPending}
                            onClick={() => action.mutate({ id: c.id, op: "start" })}
                          >
                            <Play className="size-4" />
                          </Button>
                        )}
                        <Button
                          size="icon"
                          variant="ghost"
                          className="text-red-600"
                          title="Remover"
                          disabled={action.isPending}
                          onClick={() => action.mutate({ id: c.id, op: "remove" })}
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>

      {logsFor && <LogsDialog container={logsFor} onClose={() => setLogsFor(null)} />}
    </Card>
  );
}

function LogsDialog({ container, onClose }: { container: InfraContainer; onClose: () => void }) {
  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle className="font-mono text-sm">{container.name}</DialogTitle>
        </DialogHeader>
        <ContainerLogsPane containerId={container.id} />
      </DialogContent>
    </Dialog>
  );
}
