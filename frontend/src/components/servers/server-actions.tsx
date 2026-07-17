"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, ApiError, type ManagedServer } from "@/lib/api";
import { Play, Square, RotateCw, Trash2 } from "lucide-react";

function useServerAction(
  fn: (id: string) => Promise<unknown>,
  successMessage: (server: ManagedServer) => string
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (server: ManagedServer) => fn(server.id).then(() => server),
    onSuccess: (server) => {
      queryClient.invalidateQueries({ queryKey: ["servers"] });
      toast.success(successMessage(server));
    },
    onError: (err) => {
      const message = err instanceof ApiError ? err.message : "Ação falhou";
      toast.error(message);
    },
  });
}

export function ServerActions({ server }: { server: ManagedServer }) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [keepVolume, setKeepVolume] = useState(true);

  const start = useServerAction(api.startServer, (s) => `${s.name} iniciado`);
  const stop = useServerAction(api.stopServer, (s) => `${s.name} parado`);
  const restart = useServerAction(api.restartServer, (s) => `${s.name} reiniciado`);

  const queryClient = useQueryClient();
  const remove = useMutation({
    mutationFn: () => api.deleteServer(server.id, keepVolume),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["servers"] });
      toast.success(`${server.name} excluído`);
      setConfirmDelete(false);
    },
    onError: (err) => {
      const message = err instanceof ApiError ? err.message : "Falha ao excluir";
      toast.error(message);
    },
  });

  // "creating"/"removing" = container ainda não existe (ou já foi embora) no
  // Docker — chamar start/stop/restart nesse meio-tempo vira erro confuso lá
  // no backend, então trava aqui antes mesmo de deixar clicar.
  const noContainerYet = server.status === "creating" || server.status === "removing";
  const busy = start.isPending || stop.isPending || restart.isPending || noContainerYet;
  const lifecycleTitle = noContainerYet ? "Aguarde o provisionamento terminar" : undefined;

  return (
    <div className="flex items-center gap-1">
      {server.status !== "running" ? (
        <Button
          size="icon"
          variant="ghost"
          title={lifecycleTitle ?? "Iniciar"}
          disabled={busy}
          onClick={() => start.mutate(server)}
        >
          <Play className="size-4" />
        </Button>
      ) : (
        <Button
          size="icon"
          variant="ghost"
          title={lifecycleTitle ?? "Parar"}
          disabled={busy}
          onClick={() => stop.mutate(server)}
        >
          <Square className="size-4" />
        </Button>
      )}
      <Button
        size="icon"
        variant="ghost"
        title={lifecycleTitle ?? "Reiniciar"}
        disabled={busy}
        onClick={() => restart.mutate(server)}
      >
        <RotateCw className="size-4" />
      </Button>
      <Button
        size="icon"
        variant="ghost"
        title="Excluir"
        className="text-red-600 hover:text-red-700"
        onClick={() => setConfirmDelete(true)}
      >
        <Trash2 className="size-4" />
      </Button>

      <Dialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Excluir &ldquo;{server.name}&rdquo;?</DialogTitle>
            <DialogDescription>
              Isso remove o container do servidor. Essa ação não tem volta.
            </DialogDescription>
          </DialogHeader>

          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={keepVolume}
              onChange={(e) => setKeepVolume(e.target.checked)}
            />
            Manter o volume de dados (permite recriar o servidor depois)
          </label>

          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmDelete(false)}>
              Cancelar
            </Button>
            <Button
              variant="destructive"
              disabled={remove.isPending}
              onClick={() => remove.mutate()}
            >
              {remove.isPending ? "Excluindo..." : "Excluir definitivamente"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
