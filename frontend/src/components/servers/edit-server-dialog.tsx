"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type ManagedServer } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Pencil } from "lucide-react";

export function EditServerDialog({ server }: { server: ManagedServer }) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(server.name);
  const [cpuCores, setCpuCores] = useState(server.resources.cpu_cores);
  const [memoryMb, setMemoryMb] = useState(server.resources.memory_mb);
  const [diskGb, setDiskGb] = useState(server.resources.disk_gb);
  const [hostPort, setHostPort] = useState(server.host_port);

  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: () =>
      api.updateServer(server.id, {
        name: name.trim() !== server.name ? name.trim() : undefined,
        resources:
          cpuCores !== server.resources.cpu_cores ||
          memoryMb !== server.resources.memory_mb ||
          diskGb !== server.resources.disk_gb
            ? { cpu_cores: cpuCores, memory_mb: memoryMb, disk_gb: diskGb }
            : undefined,
        host_port: hostPort !== server.host_port ? hostPort : undefined,
      }),
    onSuccess: () => {
      toast.success("Servidor atualizado");
      setOpen(false);
      queryClient.invalidateQueries({ queryKey: ["servers"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar servidor"),
  });

  const portChanged = hostPort !== server.host_port;

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" />}>
        <Pencil className="size-4" />
        Editar
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Editar servidor</DialogTitle>
          <DialogDescription>
            Nome e recursos aplicam na hora, sem reiniciar. Trocar a porta recria o
            container preservando o volume — fica alguns segundos sem aceitar conexão.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3 py-2">
          <div className="grid gap-1.5">
            <Label>Nome</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div className="grid gap-1.5">
              <Label>CPU (cores)</Label>
              <Input
                type="number"
                min={1}
                step="0.5"
                value={cpuCores}
                onChange={(e) => setCpuCores(Number(e.target.value))}
              />
            </div>
            <div className="grid gap-1.5">
              <Label>RAM (MB)</Label>
              <Input
                type="number"
                min={256}
                value={memoryMb}
                onChange={(e) => setMemoryMb(Number(e.target.value))}
              />
            </div>
            <div className="grid gap-1.5">
              <Label>Disco (GB)</Label>
              <Input type="number" min={1} value={diskGb} onChange={(e) => setDiskGb(Number(e.target.value))} />
            </div>
          </div>
          <div className="grid gap-1.5">
            <Label>Porta</Label>
            <Input type="number" value={hostPort} onChange={(e) => setHostPort(Number(e.target.value))} />
            {portChanged && (
              <p className="text-amber-600 text-xs">
                Vai recriar o container (breve interrupção) pra trocar a porta publicada.
              </p>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button disabled={mutation.isPending || !name.trim()} onClick={() => mutation.mutate()}>
            {mutation.isPending ? "Salvando..." : "Salvar"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
