"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type BackupStorageKind } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
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
import { CalendarClock, Plus, Play, Trash2 } from "lucide-react";

const WEEKDAYS = ["domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"];

export function BackupPolicies({
  serverId,
  database,
  gdriveConnected,
}: {
  serverId: string;
  database: string;
  gdriveConnected: boolean;
}) {
  const { data: policies, isLoading } = useQuery({
    queryKey: ["servers", serverId, "backup-policies"],
    queryFn: () => api.listBackupPolicies(serverId),
  });

  const queryClient = useQueryClient();
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "backup-policies"] });

  const [open, setOpen] = useState(false);
  const [storage, setStorage] = useState<BackupStorageKind>("local");
  const [frequency, setFrequency] = useState<"daily" | "weekly">("daily");
  const [weekday, setWeekday] = useState("0");
  const [timeOfDay, setTimeOfDay] = useState("03:00");
  const [retentionCount, setRetentionCount] = useState(7);

  const create = useMutation({
    mutationFn: () =>
      api.createBackupPolicy(serverId, {
        database_name: database,
        storage,
        frequency,
        weekday: frequency === "weekly" ? Number(weekday) : null,
        time_of_day: timeOfDay,
        retention_count: retentionCount,
      }),
    onSuccess: () => {
      toast.success("Política de backup criada");
      setOpen(false);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar política"),
  });

  const remove = useMutation({
    mutationFn: (policyId: string) => api.deleteBackupPolicy(serverId, policyId),
    onSuccess: () => {
      toast.success("Política excluída");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir política"),
  });

  const toggle = useMutation({
    mutationFn: (args: { policyId: string; enabled: boolean }) =>
      api.setBackupPolicyEnabled(serverId, args.policyId, args.enabled),
    onSuccess: invalidate,
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar política"),
  });

  const run = useMutation({
    mutationFn: (policyId: string) => api.runBackupPolicy(serverId, policyId),
    onSuccess: () => {
      toast.success("Backup disparado");
      setTimeout(() => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "backups"] }), 1500);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao rodar política"),
  });

  return (
    <Card>
      <CardContent className="p-0">
        <div className="flex items-center justify-between border-b p-3">
          <span className="flex items-center gap-1.5 text-sm font-medium">
            <CalendarClock className="size-4" />
            Rotina agendada de backup
          </span>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger render={<Button size="sm" variant="outline" />}>
              <Plus className="size-4" />
              Nova política
            </DialogTrigger>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Nova política de backup</DialogTitle>
              </DialogHeader>
              <div className="grid gap-3 py-2">
                <p className="text-muted-foreground text-xs">
                  Banco: <span className="font-mono">{database}</span>. Roda sozinho no horário
                  configurado (checagem a cada minuto).
                </p>
                <div className="grid grid-cols-2 gap-3">
                  <div className="grid gap-1.5">
                    <Label>Frequência</Label>
                    <Select value={frequency} onValueChange={(v) => v && setFrequency(v as typeof frequency)}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="daily">Diária</SelectItem>
                        <SelectItem value="weekly">Semanal</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  {frequency === "weekly" && (
                    <div className="grid gap-1.5">
                      <Label>Dia da semana</Label>
                      <Select value={weekday} onValueChange={(v) => v && setWeekday(v)}>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {WEEKDAYS.map((w, i) => (
                            <SelectItem key={i} value={String(i)}>
                              {w}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                  <div className="grid gap-1.5">
                    <Label>Horário (UTC)</Label>
                    <Input
                      type="time"
                      value={timeOfDay}
                      onChange={(e) => setTimeOfDay(e.target.value)}
                    />
                  </div>
                  <div className="grid gap-1.5">
                    <Label>Manter últimos N</Label>
                    <Input
                      type="number"
                      min={1}
                      value={retentionCount}
                      onChange={(e) => setRetentionCount(Number(e.target.value))}
                    />
                  </div>
                </div>
                <div className="grid gap-1.5">
                  <Label>Storage</Label>
                  <Select value={storage} onValueChange={(v) => v && setStorage(v as BackupStorageKind)}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="local">Local</SelectItem>
                      <SelectItem value="gdrive" disabled={!gdriveConnected}>
                        Google Drive {!gdriveConnected && "(conecte primeiro)"}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button disabled={create.isPending} onClick={() => create.mutate()}>
                  {create.isPending ? "Criando..." : "Criar"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !policies || policies.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhuma política de backup configurada.</p>
        ) : (
          <ul className="divide-y">
            {policies.map((p) => (
              <li key={p.id} className="flex items-start justify-between gap-3 p-3 text-sm">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-mono text-xs font-medium">{p.database_name}</span>
                    <Badge variant="outline">
                      {p.frequency === "daily" ? "diária" : `semanal · ${WEEKDAYS[p.weekday ?? 0]}`}
                    </Badge>
                    <Badge variant="outline">{p.time_of_day} UTC</Badge>
                    <Badge variant="outline">{p.storage === "gdrive" ? "Google Drive" : "local"}</Badge>
                    <Badge variant="outline">mantém {p.retention_count}</Badge>
                    {!p.enabled && <Badge variant="outline">pausada</Badge>}
                  </div>
                  <p className="text-muted-foreground text-xs">
                    {p.last_run_at
                      ? `última execução: ${new Date(p.last_run_at).toLocaleString("pt-BR")} — ${p.last_run_status === "ok" ? "ok" : "erro"}`
                      : "nunca executada"}
                    {p.last_run_error && <span className="text-red-600"> · {p.last_run_error}</span>}
                  </p>
                </div>
                <div className="flex shrink-0 gap-1">
                  <Button
                    size="icon"
                    variant="ghost"
                    title="Rodar agora"
                    disabled={run.isPending}
                    onClick={() => run.mutate(p.id)}
                  >
                    <Play className="size-4" />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    disabled={toggle.isPending}
                    onClick={() => toggle.mutate({ policyId: p.id, enabled: !p.enabled })}
                  >
                    {p.enabled ? "Pausar" : "Retomar"}
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="text-red-600"
                    onClick={() => remove.mutate(p.id)}
                  >
                    <Trash2 className="size-4" />
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
