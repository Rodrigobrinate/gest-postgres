"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { HardDrive, Plus, Trash2 } from "lucide-react";

function formatBytes(bytes?: number) {
  if (!bytes) return "—";
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
  return `${(bytes / 1024).toFixed(0)} KB`;
}

export function VolumesTab() {
  const { data: volumes, isLoading } = useQuery({
    queryKey: ["infra-volumes"],
    queryFn: () => api.listInfraVolumes(),
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["infra-volumes"] });

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");

  const create = useMutation({
    mutationFn: () => api.createInfraVolume(name),
    onSuccess: () => {
      toast.success("Volume criado");
      setOpen(false);
      setName("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar volume"),
  });

  const remove = useMutation({
    mutationFn: (volName: string) => api.removeInfraVolume(volName),
    onSuccess: () => {
      toast.success("Volume removido");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover volume"),
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <HardDrive className="size-4" />
          Volumes
        </CardTitle>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" />}>
            <Plus className="size-4" />
            Novo volume
          </DialogTrigger>
          <DialogContent className="sm:max-w-sm">
            <DialogHeader>
              <DialogTitle>Novo volume</DialogTitle>
            </DialogHeader>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="meu-volume" />
            <DialogFooter>
              <Button disabled={create.isPending || !name.trim()} onClick={() => create.mutate()}>
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !volumes || volumes.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhum volume.</p>
        ) : (
          <ul className="divide-y">
            {volumes.map((v) => (
              <li key={v.name} className="flex items-center justify-between px-4 py-2 text-sm">
                <span className="truncate font-mono text-xs">{v.name}</span>
                <div className="flex items-center gap-3">
                  <span className="text-muted-foreground text-xs">{formatBytes(v.size_bytes)}</span>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="text-red-600"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(v.name)}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
