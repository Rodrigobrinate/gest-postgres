"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, ApiError, type ContainerDetail } from "@/lib/api";
import { Cpu } from "lucide-react";

function formatDate(iso?: string) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("pt-BR");
}

export function OverviewTab({
  detail,
  containerId,
}: {
  detail: ContainerDetail;
  containerId: string;
}) {
  const labelEntries = Object.entries(detail.labels ?? {});

  const queryClient = useQueryClient();
  const [cpu, setCpu] = useState(detail.cpu_cores ? String(detail.cpu_cores) : "");
  const [memory, setMemory] = useState(detail.memory_mb ? String(detail.memory_mb) : "");

  const updateResources = useMutation({
    mutationFn: () => api.updateContainerResources(containerId, cpu ? Number(cpu) : 0, memory ? Number(memory) : 0),
    onSuccess: () => {
      toast.success("Recursos atualizados");
      queryClient.invalidateQueries({ queryKey: ["infra-container-detail", containerId] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar recursos"),
  });

  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Detalhes</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <div className="text-muted-foreground">ID</div>
          <div className="truncate font-mono text-xs">{detail.id}</div>
          <div className="text-muted-foreground">Imagem</div>
          <div className="font-mono text-xs">{detail.image}</div>
          <div className="text-muted-foreground">Criado em</div>
          <div>{formatDate(detail.created_at)}</div>
          <div className="text-muted-foreground">Iniciado em</div>
          <div>{formatDate(detail.started_at)}</div>
          {!detail.running && (
            <>
              <div className="text-muted-foreground">Finalizado em</div>
              <div>{formatDate(detail.finished_at)}</div>
              <div className="text-muted-foreground">Exit code</div>
              <div>{detail.exit_code}</div>
            </>
          )}
          <div className="text-muted-foreground">Restart policy</div>
          <div className="font-mono text-xs">{detail.restart_policy || "—"}</div>
          <div className="text-muted-foreground">Comando</div>
          <div className="font-mono text-xs">{detail.command?.join(" ") || "—"}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-1.5">
            <Cpu className="size-4" />
            Recursos
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap items-end gap-3">
          <div className="grid gap-1.5">
            <Label>CPU (cores)</Label>
            <Input
              type="number"
              step="0.1"
              min="0"
              value={cpu}
              onChange={(e) => setCpu(e.target.value)}
              placeholder="sem limite"
              className="w-32"
            />
          </div>
          <div className="grid gap-1.5">
            <Label>Memória (MB)</Label>
            <Input
              type="number"
              step="1"
              min="0"
              value={memory}
              onChange={(e) => setMemory(e.target.value)}
              placeholder="sem limite"
              className="w-32"
            />
          </div>
          <Button disabled={updateResources.isPending} onClick={() => updateResources.mutate()}>
            {updateResources.isPending ? "Aplicando..." : "Aplicar"}
          </Button>
          <p className="text-muted-foreground w-full text-xs">
            Aplica na hora, sem recriar o container nem derrubar conexão. Deixar em branco remove o
            limite.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Labels</CardTitle>
        </CardHeader>
        <CardContent>
          {labelEntries.length === 0 ? (
            <p className="text-muted-foreground text-sm">Sem labels.</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {labelEntries.map(([k, v]) => (
                <Badge key={k} variant="outline" className="font-mono text-xs">
                  {k}={v}
                </Badge>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
