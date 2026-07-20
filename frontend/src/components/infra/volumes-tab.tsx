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
import { FileBrowser, type FileBrowserAdapter } from "@/components/infra/file-browser";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Archive, Download, FolderOpen, HardDrive, History, Plus, Trash2 } from "lucide-react";
import type { VolumeBackup } from "@/lib/api";

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

  const [browsing, setBrowsing] = useState<string | null>(null);
  const [backingUp, setBackingUp] = useState<string | null>(null);

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
                  <Button size="icon" variant="ghost" title="Arquivos" onClick={() => setBrowsing(v.name)}>
                    <FolderOpen className="size-3.5" />
                  </Button>
                  <Button size="icon" variant="ghost" title="Backup" onClick={() => setBackingUp(v.name)}>
                    <Archive className="size-3.5" />
                  </Button>
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

      {browsing && (
        <Dialog open onOpenChange={(v) => !v && setBrowsing(null)}>
          <DialogContent className="sm:max-w-3xl">
            <DialogHeader>
              <DialogTitle className="font-mono text-sm">Arquivos — {browsing}</DialogTitle>
            </DialogHeader>
            <VolumeFileBrowser volumeName={browsing} />
          </DialogContent>
        </Dialog>
      )}

      {backingUp && (
        <Dialog open onOpenChange={(v) => !v && setBackingUp(null)}>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle className="font-mono text-sm">Backups — {backingUp}</DialogTitle>
            </DialogHeader>
            <VolumeBackups volumeName={backingUp} />
          </DialogContent>
        </Dialog>
      )}
    </Card>
  );
}

function VolumeBackups({ volumeName }: { volumeName: string }) {
  const { data: backups, isLoading } = useQuery({
    queryKey: ["volume-backups", volumeName],
    queryFn: () => api.listVolumeBackups(volumeName),
    refetchInterval: 5_000,
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["volume-backups", volumeName] });

  const create = useMutation({
    mutationFn: () => api.createVolumeBackup(volumeName),
    onSuccess: () => {
      toast.success("Backup criado");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar backup"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteVolumeBackup(volumeName, id),
    onSuccess: () => {
      toast.success("Backup removido");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover backup"),
  });

  const [restoring, setRestoring] = useState<VolumeBackup | null>(null);

  return (
    <div className="grid gap-3">
      <Button size="sm" className="justify-self-start" disabled={create.isPending} onClick={() => create.mutate()}>
        <Archive className="size-4" />
        {create.isPending ? "Gerando snapshot..." : "Backup agora"}
      </Button>

      {isLoading ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
      ) : !backups || backups.length === 0 ? (
        <p className="text-muted-foreground text-sm">Nenhum backup ainda.</p>
      ) : (
        <ul className="divide-y rounded-md border">
          {backups.map((b) => (
            <li key={b.id} className="flex items-center justify-between px-3 py-2 text-sm">
              <div className="flex items-center gap-2">
                <span className="font-mono text-xs">{b.filename}</span>
                <Badge
                  className={
                    b.status === "completed"
                      ? "bg-emerald-600 text-white"
                      : b.status === "failed"
                        ? "bg-red-600 text-white"
                        : undefined
                  }
                  variant={b.status === "running" ? "secondary" : undefined}
                >
                  {b.status}
                </Badge>
                <span className="text-muted-foreground text-xs">{formatBytes(b.size_bytes)}</span>
              </div>
              <div className="flex items-center gap-1">
                {b.status === "completed" && (
                  <>
                    <Button
                      size="icon-xs"
                      variant="ghost"
                      title="Baixar"
                      render={<a href={api.volumeBackupDownloadUrl(volumeName, b.id)} />}
                    >
                      <Download className="size-3.5" />
                    </Button>
                    <Button size="icon-xs" variant="ghost" title="Restaurar" onClick={() => setRestoring(b)}>
                      <History className="size-3.5" />
                    </Button>
                  </>
                )}
                <Button
                  size="icon-xs"
                  variant="ghost"
                  className="text-red-600"
                  disabled={remove.isPending}
                  onClick={() => remove.mutate(b.id)}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}

      {restoring && (
        <RestoreVolumeBackupDialog
          volumeName={volumeName}
          backup={restoring}
          onClose={() => setRestoring(null)}
        />
      )}
    </div>
  );
}

function RestoreVolumeBackupDialog({
  volumeName,
  backup,
  onClose,
}: {
  volumeName: string;
  backup: VolumeBackup;
  onClose: () => void;
}) {
  const [mode, setMode] = useState<"new" | "existing">("new");
  const [newVolumeName, setNewVolumeName] = useState(`${volumeName}_restored`);
  const [targetVolume, setTargetVolume] = useState("");

  const { data: volumes } = useQuery({
    queryKey: ["infra-volumes"],
    queryFn: () => api.listInfraVolumes(),
  });

  const queryClient = useQueryClient();

  const restore = useMutation({
    mutationFn: () =>
      api.restoreVolumeBackup(
        volumeName,
        backup.id,
        mode === "new" ? newVolumeName : targetVolume,
        mode === "new"
      ),
    onSuccess: () => {
      toast.success("Volume restaurado");
      queryClient.invalidateQueries({ queryKey: ["infra-volumes"] });
      onClose();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao restaurar"),
  });

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5">
            <History className="size-4" />
            Restaurar &ldquo;{backup.filename}&rdquo;
          </DialogTitle>
        </DialogHeader>
        <div className="grid gap-3 py-2">
          <div className="grid gap-1.5">
            <Label>Destino</Label>
            <Select value={mode} onValueChange={(v) => v && setMode(v as typeof mode)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="new">Criar um volume novo</SelectItem>
                <SelectItem value="existing">Sobrescrever um volume existente</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {mode === "new" ? (
            <div className="grid gap-1.5">
              <Label>Nome do volume novo</Label>
              <Input value={newVolumeName} onChange={(e) => setNewVolumeName(e.target.value)} />
            </div>
          ) : (
            <div className="grid gap-1.5">
              <Label>Volume a sobrescrever</Label>
              <Select value={targetVolume} onValueChange={(v) => v && setTargetVolume(v)}>
                <SelectTrigger>
                  <SelectValue placeholder="Selecione" />
                </SelectTrigger>
                <SelectContent>
                  {(volumes ?? []).map((v) => (
                    <SelectItem key={v.name} value={v.name}>
                      {v.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-amber-700">
                Apaga tudo que existe hoje nesse volume antes de restaurar — sem volta.
              </p>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button
            disabled={
              restore.isPending || (mode === "new" ? !newVolumeName.trim() : !targetVolume)
            }
            onClick={() => restore.mutate()}
          >
            {restore.isPending ? "Restaurando..." : "Restaurar"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function VolumeFileBrowser({ volumeName }: { volumeName: string }) {
  const adapter: FileBrowserAdapter = {
    list: (path) => api.listVolumeFiles(volumeName, path),
    stat: (path) => api.statVolumeFile(volumeName, path),
    read: (path) => api.readVolumeFile(volumeName, path),
    write: (path, content) => api.writeVolumeFile(volumeName, path, content),
    upload: (path, file) => api.uploadVolumeFile(volumeName, path, file),
    remove: (path) => api.deleteVolumeFile(volumeName, path),
    downloadUrl: (path) => api.volumeFileDownloadUrl(volumeName, path),
  };
  return <FileBrowser adapter={adapter} queryKeyPrefix={`volume-${volumeName}`} />;
}
