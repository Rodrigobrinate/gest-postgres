"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type AddHbaRuleInput } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import { Plus, Shield, Trash2 } from "lucide-react";

const TYPES = ["local", "host", "hostssl", "hostnossl"];
const METHODS = ["scram-sha-256", "md5", "trust", "reject", "password", "cert"];

export function HbaRules({ serverId }: { serverId: string }) {
  const { data: rules, isLoading } = useQuery({
    queryKey: ["servers", serverId, "hba-rules"],
    queryFn: () => api.listHbaRules(serverId),
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "hba-rules"] });

  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<AddHbaRuleInput>({
    type: "host",
    database: "all",
    user_name: "all",
    address: "",
    method: "scram-sha-256",
  });

  const add = useMutation({
    mutationFn: () => api.addHbaRule(serverId, form),
    onSuccess: () => {
      toast.success("Regra adicionada e recarregada");
      setOpen(false);
      setForm({ type: "host", database: "all", user_name: "all", address: "", method: "scram-sha-256" });
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao adicionar regra"),
  });

  const remove = useMutation({
    mutationFn: (raw: string) => api.deleteHbaRule(serverId, raw),
    onSuccess: () => {
      toast.success("Regra removida e recarregada");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover regra"),
  });

  return (
    <Card>
      <CardContent className="p-0">
        <div className="flex items-center justify-between border-b p-3">
          <span className="flex items-center gap-1.5 text-sm font-medium">
            <Shield className="size-4" />
            pg_hba.conf — regras de acesso
          </span>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger render={<Button size="sm" variant="outline" />}>
              <Plus className="size-4" />
              Nova regra
            </DialogTrigger>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Nova regra de acesso</DialogTitle>
              </DialogHeader>
              <div className="grid gap-3 py-2">
                <p className="text-muted-foreground text-xs">
                  Adiciona no final do arquivo — a primeira regra que casar é a que vale, então
                  isso nunca esconde uma regra mais restritiva já existente. Recarrega na hora, sem
                  restart.
                </p>
                <div className="grid grid-cols-2 gap-3">
                  <div className="grid gap-1.5">
                    <Label>Tipo</Label>
                    <Select value={form.type} onValueChange={(v) => v && setForm({ ...form, type: v })}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {TYPES.map((t) => (
                          <SelectItem key={t} value={t}>
                            {t}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="grid gap-1.5">
                    <Label>Método</Label>
                    <Select value={form.method} onValueChange={(v) => v && setForm({ ...form, method: v })}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {METHODS.map((m) => (
                          <SelectItem key={m} value={m}>
                            {m}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="grid gap-1.5">
                    <Label>Database</Label>
                    <Input
                      value={form.database}
                      onChange={(e) => setForm({ ...form, database: e.target.value })}
                      placeholder="all"
                    />
                  </div>
                  <div className="grid gap-1.5">
                    <Label>Usuário</Label>
                    <Input
                      value={form.user_name}
                      onChange={(e) => setForm({ ...form, user_name: e.target.value })}
                      placeholder="all"
                    />
                  </div>
                </div>
                {form.type !== "local" && (
                  <div className="grid gap-1.5">
                    <Label>Endereço / CIDR</Label>
                    <Input
                      value={form.address}
                      onChange={(e) => setForm({ ...form, address: e.target.value })}
                      placeholder="0.0.0.0/0 ou 10.0.0.0/8"
                    />
                  </div>
                )}
              </div>
              <DialogFooter>
                <Button
                  disabled={add.isPending || !form.database.trim() || !form.user_name.trim()}
                  onClick={() => add.mutate()}
                >
                  {add.isPending ? "Adicionando..." : "Adicionar"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !rules || rules.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhuma regra.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-muted-foreground border-b text-xs">
                  <th className="px-4 py-2 text-left font-normal">Tipo</th>
                  <th className="px-4 py-2 text-left font-normal">Database</th>
                  <th className="px-4 py-2 text-left font-normal">Usuário</th>
                  <th className="px-4 py-2 text-left font-normal">Endereço</th>
                  <th className="px-4 py-2 text-left font-normal">Método</th>
                  <th className="px-4 py-2 text-right font-normal"></th>
                </tr>
              </thead>
              <tbody>
                {rules.map((r) => (
                  <tr key={r.raw} className="border-b font-mono text-xs last:border-0">
                    <td className="px-4 py-2">{r.type}</td>
                    <td className="px-4 py-2">{r.database}</td>
                    <td className="px-4 py-2">{r.user_name}</td>
                    <td className="px-4 py-2">{r.address || "—"}</td>
                    <td className="px-4 py-2">{r.method}</td>
                    <td className="px-4 py-2 text-right">
                      <Button
                        size="icon"
                        variant="ghost"
                        className="size-6 text-red-600"
                        disabled={remove.isPending}
                        onClick={() => remove.mutate(r.raw)}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </td>
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
