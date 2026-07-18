"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type Backup, type BackupStorageKind } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Archive, Download, History, Loader2, Plus, RotateCcw, Trash2 } from "lucide-react";
import { BackupPolicies } from "../backup-policies";
import { GDriveConnection } from "../gdrive-connection";
import { cn } from "@/lib/utils";

function formatBytes(bytes?: number) {
  if (bytes == null) return "—";
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
}

function StatusBadge({ status }: { status: Backup["status"] }) {
  if (status === "completed") return <Badge className="bg-emerald-600 text-white">completo</Badge>;
  if (status === "failed") return <Badge className="bg-red-600 text-white">falhou</Badge>;
  return (
    <Badge variant="outline" className="gap-1">
      <Loader2 className="size-3 animate-spin" />
      rodando
    </Badge>
  );
}

export function BackupTab({ serverId, database }: { serverId: string; database: string }) {
  const { data: backups, isLoading } = useQuery({
    queryKey: ["servers", serverId, "backups"],
    queryFn: () => api.listBackups(serverId),
    refetchInterval: 5_000,
  });

  const { data: databases } = useQuery({
    queryKey: ["servers", serverId, "databases"],
    queryFn: () => api.listDatabases(serverId),
  });

  const { data: gdriveStatus } = useQuery({
    queryKey: ["gdrive-status"],
    queryFn: () => api.gdriveStatus(),
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "backups"] });

  const [open, setOpen] = useState(false);
  const [backupDatabase, setBackupDatabase] = useState(database);
  const [storage, setStorage] = useState<BackupStorageKind>("local");

  const create = useMutation({
    mutationFn: () => api.createBackup(serverId, backupDatabase, storage),
    onSuccess: () => {
      toast.success("Backup iniciado");
      setOpen(false);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao iniciar backup"),
  });

  const remove = useMutation({
    mutationFn: (backupId: string) => api.deleteBackup(serverId, backupId),
    onSuccess: () => {
      toast.success("Backup excluído");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir backup"),
  });

  const [restoring, setRestoring] = useState<Backup | null>(null);

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base flex items-center gap-1.5">
            <Archive className="size-4" />
            Backups
          </CardTitle>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger render={<Button size="sm" />}>
              <Plus className="size-4" />
              Novo backup
            </DialogTrigger>
            <DialogContent className="sm:max-w-sm">
              <DialogHeader>
                <DialogTitle>Fazer backup agora</DialogTitle>
              </DialogHeader>
              <div className="grid gap-3 py-2">
                <div className="grid gap-1.5">
                  <Label>Banco</Label>
                  <Select value={backupDatabase} onValueChange={(v) => v && setBackupDatabase(v)}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {(databases ?? [database]).map((d) => (
                        <SelectItem key={d} value={d}>
                          {d}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid gap-1.5">
                  <Label>Storage</Label>
                  <Select value={storage} onValueChange={(v) => v && setStorage(v as BackupStorageKind)}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="local">Local</SelectItem>
                      <SelectItem value="gdrive" disabled={!gdriveStatus?.connected}>
                        Google Drive {!gdriveStatus?.connected && "(conecte abaixo primeiro)"}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button disabled={create.isPending} onClick={() => create.mutate()}>
                  {create.isPending ? "Iniciando..." : "Fazer backup"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </CardHeader>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !backups || backups.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhum backup ainda.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-muted-foreground border-b text-xs">
                    <th className="px-4 py-2 text-left font-normal">Arquivo</th>
                    <th className="px-4 py-2 text-left font-normal">Banco</th>
                    <th className="px-4 py-2 text-left font-normal">Storage</th>
                    <th className="px-4 py-2 text-right font-normal">Tamanho</th>
                    <th className="px-4 py-2 text-left font-normal">Status</th>
                    <th className="px-4 py-2 text-left font-normal">Quando</th>
                    <th className="px-4 py-2 text-right font-normal">Ações</th>
                  </tr>
                </thead>
                <tbody>
                  {backups.map((b) => (
                    <tr key={b.id} className="border-b last:border-0">
                      <td className="px-4 py-2 font-mono text-xs">{b.filename}</td>
                      <td className="px-4 py-2 font-mono text-xs">{b.database_name}</td>
                      <td className="px-4 py-2">
                        <Badge variant="outline">{b.storage === "gdrive" ? "Google Drive" : "local"}</Badge>
                        {b.policy_id && (
                          <Badge variant="outline" className="ml-1">
                            agendado
                          </Badge>
                        )}
                      </td>
                      <td className="px-4 py-2 text-right font-mono text-xs">{formatBytes(b.size_bytes)}</td>
                      <td className={cn("px-4 py-2", b.error && "max-w-xs")} title={b.error}>
                        <StatusBadge status={b.status} />
                      </td>
                      <td className="text-muted-foreground px-4 py-2 text-xs">
                        {new Date(b.started_at).toLocaleString("pt-BR")}
                      </td>
                      <td className="px-4 py-2">
                        <div className="flex justify-end gap-1">
                          {b.status === "completed" && (
                            <>
                              <Button
                                size="icon"
                                variant="ghost"
                                title="Baixar"
                                render={<a href={api.downloadBackupUrl(serverId, b.id)} download />}
                              >
                                <Download className="size-4" />
                              </Button>
                              <Button
                                size="icon"
                                variant="ghost"
                                title="Restaurar"
                                onClick={() => setRestoring(b)}
                              >
                                <RotateCcw className="size-4" />
                              </Button>
                            </>
                          )}
                          <Button
                            size="icon"
                            variant="ghost"
                            className="text-red-600"
                            title="Excluir"
                            disabled={remove.isPending}
                            onClick={() => remove.mutate(b.id)}
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
      </Card>

      <BackupPolicies serverId={serverId} database={database} gdriveConnected={!!gdriveStatus?.connected} />
      <GDriveConnection />

      {restoring && (
        <RestoreDialog
          serverId={serverId}
          backup={restoring}
          databases={databases ?? []}
          onClose={() => setRestoring(null)}
        />
      )}
    </div>
  );
}

function RestoreDialog({
  serverId,
  backup,
  databases,
  onClose,
}: {
  serverId: string;
  backup: Backup;
  databases: string[];
  onClose: () => void;
}) {
  const [mode, setMode] = useState<"overwrite" | "new">("new");
  const [targetDatabase, setTargetDatabase] = useState(databases[0] ?? "");
  const [newDatabaseName, setNewDatabaseName] = useState(`${backup.database_name}_restored`);

  const restore = useMutation({
    mutationFn: () =>
      api.restoreBackup(serverId, backup.id, {
        create_new: mode === "new",
        new_database_name: mode === "new" ? newDatabaseName : undefined,
        target_database: mode === "overwrite" ? targetDatabase : undefined,
      }),
    onSuccess: () => {
      toast.success("Restaurado com sucesso");
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
                <SelectItem value="new">Criar um banco novo</SelectItem>
                <SelectItem value="overwrite">Sobrescrever um banco existente</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {mode === "new" ? (
            <div className="grid gap-1.5">
              <Label>Nome do banco novo</Label>
              <Input value={newDatabaseName} onChange={(e) => setNewDatabaseName(e.target.value)} />
            </div>
          ) : (
            <div className="grid gap-1.5">
              <Label>Banco a sobrescrever</Label>
              <Select value={targetDatabase} onValueChange={(v) => v && setTargetDatabase(v)}>
                <SelectTrigger>
                  <SelectValue placeholder="Selecione" />
                </SelectTrigger>
                <SelectContent>
                  {databases.map((d) => (
                    <SelectItem key={d} value={d}>
                      {d}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-amber-700">
                Apaga tudo que existe hoje nesse banco antes de restaurar — sem volta.
              </p>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button
            disabled={
              restore.isPending ||
              (mode === "new" ? !newDatabaseName.trim() : !targetDatabase)
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
