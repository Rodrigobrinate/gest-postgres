"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type MountInfo } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { HardDrive, Plus, TriangleAlert } from "lucide-react";

export function VolumesTab({
  mounts,
  containerId,
  isPlatformCreated,
}: {
  mounts: MountInfo[];
  containerId: string;
  isPlatformCreated: boolean;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <HardDrive className="size-4" />
          Volumes / mounts
        </CardTitle>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" />}>
            <Plus className="size-4" />
            Anexar volume
          </DialogTrigger>
          <AttachVolumeDialog
            containerId={containerId}
            isPlatformCreated={isPlatformCreated}
            onClose={() => setOpen(false)}
          />
        </Dialog>
      </CardHeader>
      <CardContent className="p-0">
        {!mounts || mounts.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhum volume montado.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-muted-foreground border-b text-xs">
                  <th className="px-4 py-2 text-left font-normal">Origem</th>
                  <th className="px-4 py-2 text-left font-normal">Destino no container</th>
                  <th className="px-4 py-2 text-left font-normal">Tipo</th>
                  <th className="px-4 py-2 text-left font-normal">Acesso</th>
                </tr>
              </thead>
              <tbody>
                {mounts.map((m) => (
                  <tr key={m.destination} className="border-b last:border-0">
                    <td className="px-4 py-2 font-mono text-xs">{m.name || m.source}</td>
                    <td className="px-4 py-2 font-mono text-xs">{m.destination}</td>
                    <td className="text-muted-foreground px-4 py-2 text-xs">{m.type}</td>
                    <td className="text-muted-foreground px-4 py-2 text-xs">{m.rw ? "leitura/escrita" : "só leitura"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function AttachVolumeDialog({
  containerId,
  isPlatformCreated,
  onClose,
}: {
  containerId: string;
  isPlatformCreated: boolean;
  onClose: () => void;
}) {
  const router = useRouter();
  const [volumeName, setVolumeName] = useState("");
  const [mountPath, setMountPath] = useState("");
  const [readOnly, setReadOnly] = useState(false);

  const { data: volumes } = useQuery({
    queryKey: ["infra-volumes"],
    queryFn: () => api.listInfraVolumes(),
  });

  const attach = useMutation({
    mutationFn: () =>
      api.attachContainerVolume(containerId, {
        volume_name: volumeName,
        mount_path: mountPath,
        read_only: readOnly,
      }),
    onSuccess: (result) => {
      toast.success("Volume anexado — container recriado com o novo ID");
      onClose();
      router.push(`/infra/containers?id=${result.id}`);
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao anexar volume"),
  });

  return (
    <DialogContent className="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Anexar volume</DialogTitle>
      </DialogHeader>
      <div className="grid gap-3 py-2">
        <div className="rounded-md border border-amber-300 bg-amber-50 p-3 text-xs text-amber-800">
          <p className="flex items-center gap-1.5 font-medium">
            <TriangleAlert className="size-3.5" />
            Isso para, remove e recria o container (breve interrupção) — o ID muda.
          </p>
          {isPlatformCreated ? (
            <p className="mt-1">
              Container criado por essa plataforma — a recriação preserva imagem, env, portas e
              redes.
            </p>
          ) : (
            <p className="mt-1">
              Container adotado de fora da plataforma — a recriação é <strong>best-effort</strong>:
              configurações mais avançadas (healthcheck, capabilities, ulimits) não são
              preservadas.
            </p>
          )}
        </div>
        <div className="grid gap-1.5">
          <Label>Volume</Label>
          <Select value={volumeName} onValueChange={(v) => v && setVolumeName(v)}>
            <SelectTrigger>
              <SelectValue placeholder="Escolher volume" />
            </SelectTrigger>
            <SelectContent>
              {(volumes ?? []).map((v) => (
                <SelectItem key={v.name} value={v.name}>
                  {v.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="grid gap-1.5">
          <Label>Caminho de montagem no container</Label>
          <Input
            value={mountPath}
            onChange={(e) => setMountPath(e.target.value)}
            placeholder="/data"
            className="font-mono text-xs"
          />
        </div>
        <label className="flex items-center gap-1.5 text-sm">
          <input type="checkbox" checked={readOnly} onChange={(e) => setReadOnly(e.target.checked)} />
          Somente leitura
        </label>
      </div>
      <DialogFooter>
        <Button
          disabled={attach.isPending || !volumeName || !mountPath.trim()}
          onClick={() => attach.mutate()}
        >
          {attach.isPending ? "Recriando..." : "Anexar e recriar"}
        </Button>
      </DialogFooter>
    </DialogContent>
  );
}
