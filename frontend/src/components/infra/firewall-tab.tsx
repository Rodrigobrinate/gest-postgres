"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
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
import { ShieldAlert, Plus, Trash2 } from "lucide-react";

export function FirewallTab() {
  const { data: rules, isLoading, error } = useQuery({
    queryKey: ["firewall-rules"],
    queryFn: () => api.listFirewallRules(),
    retry: false,
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["firewall-rules"] });

  const [open, setOpen] = useState(false);
  const [port, setPort] = useState(8080);
  const [proto, setProto] = useState<"tcp" | "udp">("tcp");
  const [action, setAction] = useState<"allow" | "deny">("allow");

  const add = useMutation({
    mutationFn: () => api.addFirewallRule(port, proto, action),
    onSuccess: () => {
      toast.success("Regra aplicada");
      setOpen(false);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao aplicar regra"),
  });

  const remove = useMutation({
    mutationFn: (r: { port: number; proto: "tcp" | "udp" }) => api.removeFirewallRule(r.port, r.proto),
    onSuccess: () => {
      toast.success("Regra removida");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover regra"),
  });

  if (error) {
    return (
      <Card>
        <CardContent className="text-muted-foreground flex flex-col items-center gap-2 p-10 text-center text-sm">
          <ShieldAlert className="size-8" />
          <p>{error instanceof ApiError ? error.message : "firewall-agent indisponível"}</p>
          <p className="text-xs">
            O agente roda fora do Docker, direto no host (systemd) — precisa do <code>setup.sh</code>{" "}
            atualizado nesse servidor pra existir.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <ShieldAlert className="size-4" />
          Firewall (ufw)
        </CardTitle>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" />}>
            <Plus className="size-4" />
            Nova regra
          </DialogTrigger>
          <DialogContent className="sm:max-w-sm">
            <DialogHeader>
              <DialogTitle>Nova regra de firewall</DialogTitle>
            </DialogHeader>
            <div className="grid gap-3 py-2">
              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-1.5">
                  <Label>Porta</Label>
                  <Input type="number" value={port} onChange={(e) => setPort(Number(e.target.value))} />
                </div>
                <div className="grid gap-1.5">
                  <Label>Protocolo</Label>
                  <Select value={proto} onValueChange={(v) => v && setProto(v as "tcp" | "udp")}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="tcp">TCP</SelectItem>
                      <SelectItem value="udp">UDP</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className="grid gap-1.5">
                <Label>Ação</Label>
                <Select value={action} onValueChange={(v) => v && setAction(v as "allow" | "deny")}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="allow">Liberar</SelectItem>
                    <SelectItem value="deny">Bloquear</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <p className="text-muted-foreground text-xs">
                Porta 22/tcp (SSH) nunca pode ser alterada por aqui — trava fixa, protege contra perder
                acesso remoto ao servidor.
              </p>
            </div>
            <DialogFooter>
              <Button disabled={add.isPending || port <= 0} onClick={() => add.mutate()}>
                {add.isPending ? "Aplicando..." : "Aplicar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !rules || rules.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhuma regra.</p>
        ) : (
          <ul className="divide-y">
            {rules.map((r) => {
              const isSsh = r.port === 22 && r.proto === "tcp";
              return (
                <li key={`${r.port}-${r.proto}-${r.action}`} className="flex items-center justify-between px-4 py-2 text-sm">
                  <div className="flex items-center gap-2">
                    <span className="font-mono">
                      {r.port}/{r.proto}
                    </span>
                    <Badge className={r.action === "allow" ? "bg-emerald-600 text-white" : "bg-red-600 text-white"}>
                      {r.action === "allow" ? "liberado" : "bloqueado"}
                    </Badge>
                    {isSsh && <Badge variant="outline">SSH — protegido</Badge>}
                  </div>
                  {!isSsh && (
                    <Button
                      size="icon"
                      variant="ghost"
                      className="text-red-600"
                      disabled={remove.isPending}
                      onClick={() => remove.mutate({ port: r.port, proto: r.proto })}
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
