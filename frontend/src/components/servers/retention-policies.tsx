"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
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
import { Archive, Plus, Play, Trash2 } from "lucide-react";

export function RetentionPolicies({ serverId, database }: { serverId: string; database: string }) {
  const { data: policies, isLoading } = useQuery({
    queryKey: ["servers", serverId, "retention-policies"],
    queryFn: () => api.listRetentionPolicies(serverId),
  });

  const queryClient = useQueryClient();
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "retention-policies"] });

  const [open, setOpen] = useState(false);
  const [schema, setSchema] = useState("public");
  const [table, setTable] = useState("");
  const [dateColumn, setDateColumn] = useState("created_at");
  const [maxAgeDays, setMaxAgeDays] = useState(365);
  const [action, setAction] = useState<"archive" | "delete">("archive");

  const create = useMutation({
    mutationFn: () =>
      api.createRetentionPolicy(serverId, {
        database_name: database,
        schema_name: schema,
        table_name: table,
        date_column: dateColumn,
        max_age_days: maxAgeDays,
        action,
      }),
    onSuccess: () => {
      toast.success("Política de retenção criada");
      setOpen(false);
      setTable("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar política"),
  });

  const remove = useMutation({
    mutationFn: (policyId: string) => api.deleteRetentionPolicy(serverId, policyId),
    onSuccess: () => {
      toast.success("Política excluída");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir política"),
  });

  const toggle = useMutation({
    mutationFn: (args: { policyId: string; enabled: boolean }) =>
      api.setRetentionPolicyEnabled(serverId, args.policyId, args.enabled),
    onSuccess: invalidate,
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar política"),
  });

  const run = useMutation({
    mutationFn: (policyId: string) => api.runRetentionPolicy(serverId, policyId),
    onSuccess: (result) => {
      toast.success(`Executado — ${result.rows_affected} linha(s) afetada(s)`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao executar política"),
  });

  return (
    <Card>
      <CardContent className="p-0">
        <div className="flex items-center justify-between border-b p-3">
          <span className="flex items-center gap-1.5 text-sm font-medium">
            <Archive className="size-4" />
            Retenção / arquivamento de dados antigos
          </span>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger render={<Button size="sm" variant="outline" />}>
              <Plus className="size-4" />
              Nova política
            </DialogTrigger>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Nova política de retenção</DialogTitle>
              </DialogHeader>
              <div className="grid gap-3 py-2">
                <p className="text-muted-foreground text-xs">
                  Roda automaticamente uma vez por dia (e pode ser disparada manualmente). Banco:{" "}
                  <span className="font-mono">{database}</span>
                </p>
                <div className="grid grid-cols-[1fr_2fr] gap-3">
                  <div className="grid gap-1.5">
                    <Label>Schema</Label>
                    <Input value={schema} onChange={(e) => setSchema(e.target.value)} />
                  </div>
                  <div className="grid gap-1.5">
                    <Label>Tabela</Label>
                    <Input value={table} onChange={(e) => setTable(e.target.value)} />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="grid gap-1.5">
                    <Label>Coluna de data</Label>
                    <Input value={dateColumn} onChange={(e) => setDateColumn(e.target.value)} />
                  </div>
                  <div className="grid gap-1.5">
                    <Label>Idade máxima (dias)</Label>
                    <Input
                      type="number"
                      value={maxAgeDays}
                      onChange={(e) => setMaxAgeDays(Number(e.target.value))}
                    />
                  </div>
                </div>
                <div className="grid gap-1.5">
                  <Label>Ação</Label>
                  <Select value={action} onValueChange={(v) => v && setAction(v as typeof action)}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="archive">Arquivar (move pra tabela_archive antes de apagar)</SelectItem>
                      <SelectItem value="delete">Apagar direto</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button
                  disabled={create.isPending || !table.trim() || !dateColumn.trim() || maxAgeDays <= 0}
                  onClick={() => create.mutate()}
                >
                  {create.isPending ? "Criando..." : "Criar"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !policies || policies.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhuma política de retenção configurada.</p>
        ) : (
          <ul className="divide-y">
            {policies.map((p) => (
              <li key={p.id} className="flex items-start justify-between gap-3 p-3 text-sm">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-mono text-xs font-medium">
                      {p.database_name}.{p.schema_name}.{p.table_name}
                    </span>
                    <Badge variant="outline">{p.action === "archive" ? "arquivar" : "apagar"}</Badge>
                    {!p.enabled && <Badge variant="outline">pausada</Badge>}
                  </div>
                  <p className="text-muted-foreground text-xs">
                    {p.date_column} &gt; {p.max_age_days} dias
                  </p>
                  <p className="text-muted-foreground text-xs">
                    {p.last_run_at
                      ? `última execução: ${new Date(p.last_run_at).toLocaleString("pt-BR")} — ${p.last_run_rows_affected ?? 0} linha(s)`
                      : "nunca executada"}
                    {p.last_run_error && <span className="text-red-600"> · erro: {p.last_run_error}</span>}
                  </p>
                </div>
                <div className="flex shrink-0 gap-1">
                  <Button
                    size="icon"
                    variant="ghost"
                    title="Executar agora"
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
