"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type AlertMetric } from "@/lib/api";
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
import { Bell, Plus, Trash2 } from "lucide-react";

const METRICS: { value: AlertMetric; label: string; unit: string; defaultThreshold: number }[] = [
  { value: "connections_pct", label: "Conexões perto do limite", unit: "%", defaultThreshold: 80 },
  { value: "disk_pct", label: "Disco enchendo", unit: "%", defaultThreshold: 80 },
  { value: "long_running_query_seconds", label: "Query travada", unit: "s", defaultThreshold: 60 },
  { value: "deadlocks", label: "Deadlocks novos", unit: "", defaultThreshold: 1 },
];

export function AlertRules({ serverId }: { serverId: string }) {
  const { data: rules, isLoading } = useQuery({
    queryKey: ["servers", serverId, "alert-rules"],
    queryFn: () => api.listAlertRules(serverId),
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "alert-rules"] });

  const [open, setOpen] = useState(false);
  const [metric, setMetric] = useState<AlertMetric>("connections_pct");
  const [threshold, setThreshold] = useState(80);
  const [webhookUrl, setWebhookUrl] = useState("");

  const create = useMutation({
    mutationFn: () => api.createAlertRule(serverId, { metric, threshold, webhook_url: webhookUrl }),
    onSuccess: () => {
      toast.success("Alerta criado");
      setOpen(false);
      setWebhookUrl("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar alerta"),
  });

  const remove = useMutation({
    mutationFn: (ruleId: string) => api.deleteAlertRule(serverId, ruleId),
    onSuccess: () => {
      toast.success("Alerta excluído");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir alerta"),
  });

  const toggle = useMutation({
    mutationFn: (args: { ruleId: string; enabled: boolean }) =>
      api.setAlertRuleEnabled(serverId, args.ruleId, args.enabled),
    onSuccess: invalidate,
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar alerta"),
  });

  return (
    <Card>
      <CardContent className="p-0">
        <div className="flex items-center justify-between border-b p-3">
          <span className="flex items-center gap-1.5 text-sm font-medium">
            <Bell className="size-4" />
            Alertas (webhook)
          </span>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger render={<Button size="sm" variant="outline" />}>
              <Plus className="size-4" />
              Novo alerta
            </DialogTrigger>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Nova regra de alerta</DialogTitle>
              </DialogHeader>
              <div className="grid gap-3 py-2">
                <div className="grid gap-1.5">
                  <Label>Métrica</Label>
                  <Select
                    value={metric}
                    onValueChange={(v) => {
                      if (!v) return;
                      const m = v as AlertMetric;
                      setMetric(m);
                      setThreshold(METRICS.find((x) => x.value === m)?.defaultThreshold ?? 1);
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {METRICS.map((m) => (
                        <SelectItem key={m.value} value={m.value}>
                          {m.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid gap-1.5">
                  <Label>Threshold ({METRICS.find((m) => m.value === metric)?.unit || "un."})</Label>
                  <Input type="number" value={threshold} onChange={(e) => setThreshold(Number(e.target.value))} />
                </div>
                <div className="grid gap-1.5">
                  <Label>Webhook URL</Label>
                  <Input
                    placeholder="https://hooks.slack.com/... ou qualquer endpoint que aceite POST JSON"
                    value={webhookUrl}
                    onChange={(e) => setWebhookUrl(e.target.value)}
                  />
                </div>
                <p className="text-muted-foreground text-xs">
                  Checado a cada minuto, com cooldown de 15min entre disparos da mesma regra.
                </p>
              </div>
              <DialogFooter>
                <Button disabled={create.isPending || !webhookUrl.trim()} onClick={() => create.mutate()}>
                  {create.isPending ? "Criando..." : "Criar"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !rules || rules.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhum alerta configurado.</p>
        ) : (
          <ul className="divide-y">
            {rules.map((r) => {
              const meta = METRICS.find((m) => m.value === r.metric);
              return (
                <li key={r.id} className="flex items-start justify-between gap-3 p-3 text-sm">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{meta?.label ?? r.metric}</span>
                      <Badge variant="outline">
                        &gt;= {r.threshold}
                        {meta?.unit}
                      </Badge>
                      {!r.enabled && <Badge variant="outline">pausado</Badge>}
                    </div>
                    <p className="text-muted-foreground truncate text-xs">{r.webhook_url}</p>
                    <p className="text-muted-foreground text-xs">
                      {r.last_value != null && `último valor: ${r.last_value.toFixed(1)}${meta?.unit ?? ""} · `}
                      {r.last_triggered_at
                        ? `disparado em ${new Date(r.last_triggered_at).toLocaleString("pt-BR")}`
                        : "nunca disparado"}
                    </p>
                  </div>
                  <div className="flex shrink-0 gap-1">
                    <Button
                      size="sm"
                      variant="ghost"
                      disabled={toggle.isPending}
                      onClick={() => toggle.mutate({ ruleId: r.id, enabled: !r.enabled })}
                    >
                      {r.enabled ? "Pausar" : "Retomar"}
                    </Button>
                    <Button size="icon" variant="ghost" className="text-red-600" onClick={() => remove.mutate(r.id)}>
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
