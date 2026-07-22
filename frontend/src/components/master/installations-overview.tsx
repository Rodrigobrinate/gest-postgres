"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { masterApi } from "@/lib/master-api";
import { useSelectedServer } from "@/lib/server-context";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { LogoutButton } from "@/components/auth/logout-button";
import { CreateInstallationDialog } from "./create-installation-dialog";
import { EditInstallationDialog } from "./edit-installation-dialog";
import { Database, Trash2 } from "lucide-react";

// Tela inicial em MULTI_SERVER_MODE (hospedado no Cloudflare Pages, atrás
// do Worker do sistema mestre) enquanto nenhuma instalação foi selecionada
// — métricas básicas de TODAS as instalações gest-postgres cadastradas,
// clicar numa entra no dashboard de sempre (intocado) escopado só àquela
// instalação. "Instalação" aqui = um droplet/servidor gest-postgres inteiro
// (backend + Postgres gerenciados dele), não confundir com a lista de
// servidores Postgres que já existe DENTRO de cada instalação.
export function InstallationsOverview() {
  const { selectServer } = useSelectedServer();
  const queryClient = useQueryClient();
  const { data, isLoading, error } = useQuery({
    queryKey: ["master-servers"],
    queryFn: masterApi.listServers,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => masterApi.deleteServer(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["master-servers"] });
      toast.success("Instalação removida");
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : "Falha ao remover"),
  });

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 p-6 sm:p-10">
      <header className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="bg-primary text-primary-foreground flex size-10 items-center justify-center rounded-lg">
            <Database className="size-5" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">gest-postgres</h1>
            <p className="text-muted-foreground text-sm">Escolha uma instalação</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <CreateInstallationDialog />
          <LogoutButton />
        </div>
      </header>

      {isLoading && <p className="text-muted-foreground text-sm">Carregando instalações...</p>}
      {error && (
        <p className="text-destructive text-sm">
          Falha ao listar instalações: {error instanceof Error ? error.message : "erro desconhecido"}
        </p>
      )}
      {data?.length === 0 && (
        <p className="text-muted-foreground text-sm">Nenhuma instalação cadastrada ainda.</p>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {data?.map((s) => (
          <Card
            key={s.id}
            className="hover:border-primary cursor-pointer transition-colors"
            onClick={() => selectServer(s.id)}
          >
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-base">{s.name}</CardTitle>
              <div className="flex items-center gap-2">
                <Badge variant={s.online ? "default" : "destructive"}>
                  {s.online ? "online" : "offline"}
                </Badge>
                <EditInstallationDialog installation={s} />
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-6"
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteMutation.mutate(s.id);
                  }}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-1">
              <p className="text-muted-foreground truncate font-mono text-xs">{s.tunnel_hostname}</p>
              {s.version && <p className="text-muted-foreground text-sm">versão {s.version}</p>}
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
