"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type CronFrequency, type CronJob } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Clock, Play, Plus, Trash2 } from "lucide-react";

const WEEKDAYS = ["domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"];

function describeFrequency(job: CronJob) {
  if (job.frequency === "interval") return `a cada ${job.interval_minutes}min`;
  if (job.frequency === "weekly") return `${WEEKDAYS[job.weekday ?? 0]} às ${job.time_of_day} UTC`;
  return `diário às ${job.time_of_day} UTC`;
}

export function CronTab({ containerId, containerName }: { containerId: string; containerName: string }) {
  const { data: jobs, isLoading } = useQuery({
    queryKey: ["cron-jobs", containerId],
    queryFn: () => api.listCronJobs(containerId),
    refetchInterval: 15_000,
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["cron-jobs", containerId] });

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [command, setCommand] = useState("");
  const [frequency, setFrequency] = useState<CronFrequency>("interval");
  const [intervalMinutes, setIntervalMinutes] = useState("60");
  const [weekday, setWeekday] = useState("1");
  const [timeOfDay, setTimeOfDay] = useState("03:00");

  const create = useMutation({
    mutationFn: () =>
      api.createCronJob({
        container_id: containerId,
        container_name: containerName,
        name,
        command,
        frequency,
        interval_minutes: frequency === "interval" ? Number(intervalMinutes) : undefined,
        weekday: frequency === "weekly" ? Number(weekday) : undefined,
        time_of_day: frequency !== "interval" ? timeOfDay : undefined,
      }),
    onSuccess: () => {
      toast.success("Cron job criado");
      setOpen(false);
      setName("");
      setCommand("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar cron job"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteCronJob(id),
    onSuccess: () => {
      toast.success("Cron job removido");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover"),
  });

  const toggle = useMutation({
    mutationFn: (job: CronJob) => api.setCronJobEnabled(job.id, !job.enabled),
    onSuccess: invalidate,
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar"),
  });

  const runNow = useMutation({
    mutationFn: (id: string) => api.runCronJobNow(id),
    onSuccess: () => {
      toast.success("Executado");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao executar"),
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <Clock className="size-4" />
          Cron jobs
        </CardTitle>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" />}>
            <Plus className="size-4" />
            Novo cron job
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Novo cron job</DialogTitle>
            </DialogHeader>
            <div className="grid gap-3 py-2">
              <div className="grid gap-1.5">
                <Label>Nome</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="limpar cache" />
              </div>
              <div className="grid gap-1.5">
                <Label>Comando (shell)</Label>
                <textarea
                  value={command}
                  onChange={(e) => setCommand(e.target.value)}
                  className="h-20 resize-none rounded-md border bg-background p-2 font-mono text-xs"
                  placeholder="php artisan schedule:run"
                  spellCheck={false}
                />
              </div>
              <div className="grid gap-1.5">
                <Label>Frequência</Label>
                <Select value={frequency} onValueChange={(v) => v && setFrequency(v as CronFrequency)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="interval">A cada N minutos</SelectItem>
                    <SelectItem value="daily">Diário</SelectItem>
                    <SelectItem value="weekly">Semanal</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {frequency === "interval" && (
                <div className="grid gap-1.5">
                  <Label>Intervalo (minutos)</Label>
                  <Input
                    type="number"
                    min="1"
                    value={intervalMinutes}
                    onChange={(e) => setIntervalMinutes(e.target.value)}
                  />
                </div>
              )}
              {frequency === "weekly" && (
                <div className="grid gap-1.5">
                  <Label>Dia da semana</Label>
                  <Select value={weekday} onValueChange={(v) => v && setWeekday(v)}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {WEEKDAYS.map((d, i) => (
                        <SelectItem key={i} value={String(i)}>
                          {d}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
              {frequency !== "interval" && (
                <div className="grid gap-1.5">
                  <Label>Horário (UTC)</Label>
                  <Input type="time" value={timeOfDay} onChange={(e) => setTimeOfDay(e.target.value)} />
                </div>
              )}
            </div>
            <DialogFooter>
              <Button
                disabled={create.isPending || !name.trim() || !command.trim()}
                onClick={() => create.mutate()}
              >
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !jobs || jobs.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhum cron job.</p>
        ) : (
          <ul className="divide-y">
            {jobs.map((job) => (
              <li key={job.id} className="flex flex-col gap-1 px-4 py-3 text-sm">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{job.name}</span>
                    <Badge variant="outline">{describeFrequency(job)}</Badge>
                    {!job.enabled && <Badge variant="secondary">desabilitado</Badge>}
                    {job.last_exit_code != null && (
                      <Badge className={job.last_exit_code === 0 ? "bg-emerald-600 text-white" : "bg-red-600 text-white"}>
                        exit {job.last_exit_code}
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      size="icon-xs"
                      variant="ghost"
                      title="Rodar agora"
                      disabled={runNow.isPending}
                      onClick={() => runNow.mutate(job.id)}
                    >
                      <Play className="size-3.5" />
                    </Button>
                    <Button size="xs" variant="outline" disabled={toggle.isPending} onClick={() => toggle.mutate(job)}>
                      {job.enabled ? "Desabilitar" : "Habilitar"}
                    </Button>
                    <Button
                      size="icon-xs"
                      variant="ghost"
                      className="text-red-600"
                      disabled={remove.isPending}
                      onClick={() => {
                        if (confirm(`Excluir cron job "${job.name}"?`)) remove.mutate(job.id);
                      }}
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                </div>
                <span className="text-muted-foreground font-mono text-xs">{job.command}</span>
                {job.last_run_at && (
                  <span className="text-muted-foreground text-xs">
                    última execução: {new Date(job.last_run_at).toLocaleString("pt-BR")}
                  </span>
                )}
                {job.last_output && (
                  <pre className="bg-muted mt-1 max-h-32 overflow-auto rounded p-2 text-xs">{job.last_output}</pre>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
