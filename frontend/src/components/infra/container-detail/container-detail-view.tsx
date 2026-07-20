"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ArrowLeft, Box, Play, RotateCw, Square, Trash2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { OverviewTab } from "./overview-tab";
import { StatsTab } from "./stats-tab";
import { EnvTab } from "./env-tab";
import { NetworksTab } from "./networks-tab";
import { VolumesTab } from "./volumes-tab";
import { FilesTab } from "./files-tab";
import { CronTab } from "./cron-tab";
import { TerminalTab } from "./terminal-tab";
import { ContainerLogsPane } from "@/components/infra/container-logs-pane";

export function ContainerDetailView({ containerId }: { containerId: string }) {
  const router = useRouter();

  const { data: detail, isLoading } = useQuery({
    queryKey: ["infra-container-detail", containerId],
    queryFn: () => api.containerDetail(containerId),
    refetchInterval: 10_000,
  });

  const queryClient = useQueryClient();
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["infra-container-detail", containerId] });

  const action = useMutation({
    mutationFn: async (op: "start" | "stop" | "restart" | "remove") => {
      if (op === "start") await api.startInfraContainer(containerId);
      else if (op === "stop") await api.stopInfraContainer(containerId);
      else if (op === "restart") await api.restartInfraContainer(containerId);
      else await api.removeInfraContainer(containerId);
    },
    onSuccess: (_d, op) => {
      if (op === "remove") {
        toast.success("Container removido");
        router.push("/infra");
        return;
      }
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha na ação"),
  });

  if (isLoading || !detail) {
    return (
      <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-6 sm:p-10">
        <p className="text-muted-foreground text-sm">Carregando...</p>
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-6 sm:p-10">
      <div>
        <Link
          href="/infra"
          className="text-muted-foreground inline-flex items-center gap-1 text-sm hover:text-foreground"
        >
          <ArrowLeft className="size-4" />
          Docker
        </Link>
      </div>

      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="bg-primary text-primary-foreground flex size-10 items-center justify-center rounded-lg">
            <Box className="size-5" />
          </div>
          <div>
            <h1 className="font-mono text-xl font-semibold tracking-tight">{detail.name}</h1>
            <div className="flex items-center gap-2">
              <span
                className={cn(
                  "text-xs font-medium",
                  detail.running ? "text-emerald-600" : "text-muted-foreground"
                )}
              >
                {detail.status}
              </span>
              <span className="text-muted-foreground text-xs font-mono">{detail.image}</span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          {detail.running ? (
            <>
              <Button
                variant="outline"
                size="sm"
                disabled={action.isPending}
                onClick={() => action.mutate("stop")}
              >
                <Square className="size-4" />
                Parar
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={action.isPending}
                onClick={() => action.mutate("restart")}
              >
                <RotateCw className="size-4" />
                Reiniciar
              </Button>
            </>
          ) : (
            <Button
              variant="outline"
              size="sm"
              disabled={action.isPending}
              onClick={() => action.mutate("start")}
            >
              <Play className="size-4" />
              Iniciar
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            className="text-red-600"
            disabled={action.isPending}
            onClick={() => {
              if (confirm(`Remover o container "${detail.name}"? O volume não é apagado.`)) {
                action.mutate("remove");
              }
            }}
          >
            <Trash2 className="size-4" />
            Remover
          </Button>
        </div>
      </header>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Visão geral</TabsTrigger>
          <TabsTrigger value="stats">Estatísticas</TabsTrigger>
          <TabsTrigger value="env">Variáveis</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="networks">Redes</TabsTrigger>
          <TabsTrigger value="volumes">Volumes</TabsTrigger>
          <TabsTrigger value="files">Arquivos</TabsTrigger>
          <TabsTrigger value="cron">Cron</TabsTrigger>
          <TabsTrigger value="terminal">Terminal</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="pt-4">
          <OverviewTab detail={detail} containerId={containerId} />
        </TabsContent>
        <TabsContent value="stats" className="pt-4">
          <StatsTab containerId={containerId} />
        </TabsContent>
        <TabsContent value="env" className="pt-4">
          <EnvTab env={detail.env} containerId={containerId} />
        </TabsContent>
        <TabsContent value="logs" className="pt-4">
          <ContainerLogsPane containerId={containerId} />
        </TabsContent>
        <TabsContent value="networks" className="pt-4">
          <NetworksTab containerId={containerId} networks={detail.networks} onChanged={invalidate} />
        </TabsContent>
        <TabsContent value="volumes" className="pt-4">
          <VolumesTab
            mounts={detail.mounts}
            containerId={containerId}
            isPlatformCreated={detail.labels?.["gestpg.infra_created"] === "true"}
          />
        </TabsContent>
        <TabsContent value="files" className="pt-4">
          <FilesTab containerId={containerId} mounts={detail.mounts} />
        </TabsContent>
        <TabsContent value="cron" className="pt-4">
          <CronTab containerId={containerId} containerName={detail.name} />
        </TabsContent>
        <TabsContent value="terminal" className="pt-4">
          <TerminalTab containerId={containerId} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
