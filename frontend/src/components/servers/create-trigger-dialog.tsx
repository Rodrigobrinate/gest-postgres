"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type CreateTriggerInput } from "@/lib/api";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Plus } from "lucide-react";

const EVENTS = ["INSERT", "UPDATE", "DELETE", "TRUNCATE"] as const;

export function CreateTriggerDialog({
  serverId,
  database,
  schema,
  table,
}: {
  serverId: string;
  database: string;
  schema: string;
  table: string;
}) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [timing, setTiming] = useState<CreateTriggerInput["timing"]>("BEFORE");
  const [events, setEvents] = useState<CreateTriggerInput["events"]>(["INSERT"]);
  const [level, setLevel] = useState<CreateTriggerInput["level"]>("ROW");
  const [functionName, setFunctionName] = useState("");

  const { data: functions } = useQuery({
    queryKey: ["servers", serverId, "trigger-functions", database],
    queryFn: () => api.listTriggerFunctions(serverId, database),
    enabled: open,
  });

  const queryClient = useQueryClient();
  const create = useMutation({
    mutationFn: () =>
      api.createTrigger(serverId, database, { name, schema, table, timing, events, level, function_name: functionName }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "triggers"] });
      toast.success(`Trigger "${name}" criado`);
      setOpen(false);
      setName("");
      setFunctionName("");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar trigger"),
  });

  function toggleEvent(ev: (typeof EVENTS)[number]) {
    setEvents((cur) => (cur.includes(ev) ? cur.filter((e) => e !== ev) : [...cur, ev]));
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return toast.error("Nome do trigger é obrigatório");
    if (!functionName.trim()) return toast.error("Escolhe a função a executar");
    if (events.length === 0) return toast.error("Escolhe pelo menos um evento");
    create.mutate();
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Plus className="size-4" />
        Novo trigger
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Criar trigger</DialogTitle>
            <DialogDescription>
              Em {schema}.{table}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="trigger-name">Nome</Label>
              <Input id="trigger-name" value={name} onChange={(e) => setName(e.target.value)} required />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label>Timing</Label>
                <Select value={timing} onValueChange={(v) => v && setTiming(v as typeof timing)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="BEFORE">BEFORE</SelectItem>
                    <SelectItem value="AFTER">AFTER</SelectItem>
                    <SelectItem value="INSTEAD OF">INSTEAD OF</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-2">
                <Label>Nível</Label>
                <Select value={level} onValueChange={(v) => v && setLevel(v as typeof level)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ROW">ROW (por linha)</SelectItem>
                    <SelectItem value="STATEMENT">STATEMENT (por comando)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="grid gap-2">
              <Label>Eventos</Label>
              <div className="flex gap-4">
                {EVENTS.map((ev) => (
                  <label key={ev} className="flex items-center gap-1.5 text-sm">
                    <input
                      type="checkbox"
                      checked={events.includes(ev)}
                      onChange={() => toggleEvent(ev)}
                    />
                    {ev}
                  </label>
                ))}
              </div>
            </div>

            <div className="grid gap-2">
              <Label>Função a executar</Label>
              {functions && functions.length > 0 ? (
                <Select value={functionName} onValueChange={(v) => v && setFunctionName(v)}>
                  <SelectTrigger>
                    <SelectValue placeholder="Escolhe a função" />
                  </SelectTrigger>
                  <SelectContent>
                    {functions.map((f) => (
                      <SelectItem key={f} value={f}>
                        {f}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <>
                  <Input
                    placeholder="schema.funcao"
                    value={functionName}
                    onChange={(e) => setFunctionName(e.target.value)}
                  />
                  <p className="text-muted-foreground text-xs">
                    Nenhuma função com retorno <code>trigger</code> encontrada — cria uma pelo
                    editor SQL primeiro (<code>CREATE FUNCTION ... RETURNS trigger</code>).
                  </p>
                </>
              )}
            </div>
          </div>

          <DialogFooter>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? "Criando..." : "Criar trigger"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
