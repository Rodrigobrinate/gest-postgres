"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type ManagedServer } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { KeyRound, Copy, Eye, EyeOff, RefreshCw } from "lucide-react";

export function ConnectionStringDialog({ server }: { server: ManagedServer }) {
  const [open, setOpen] = useState(false);
  const [reveal, setReveal] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["servers", server.id, "password"],
    queryFn: () => api.getPassword(server.id),
    enabled: open,
    staleTime: Infinity,
  });

  const queryClient = useQueryClient();
  const rotate = useMutation({
    mutationFn: () => api.rotateSuperuserPassword(server.id),
    onSuccess: (result) => {
      queryClient.setQueryData(["servers", server.id, "password"], result);
      setReveal(true);
      toast.success("Senha regenerada — connection string atualizada abaixo");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao regenerar senha"),
  });

  const host = typeof window !== "undefined" ? window.location.hostname : "localhost";
  const password = data?.password ?? "";

  const connectionString = `postgresql://${server.username}:${password}@${host}:${server.host_port}/${server.database_name}`;
  const masked = `postgresql://${server.username}:${"•".repeat(10)}@${host}:${server.host_port}/${server.database_name}`;

  function copy() {
    if (!password) return;
    navigator.clipboard.writeText(connectionString);
    toast.success("Connection string copiada");
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" />}>
        <KeyRound className="size-4" />
        Connection string
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Connection string</DialogTitle>
          <DialogDescription>
            Pra conectar de fora da plataforma (psql, DBeaver, sua aplicação, etc).
          </DialogDescription>
        </DialogHeader>

        <div className="grid grid-cols-2 gap-3 text-sm">
          <Field label="Host" value={host} />
          <Field label="Porta" value={String(server.host_port)} />
          <Field label="Usuário" value={server.username} />
          <Field label="Banco" value={server.database_name} />
        </div>

        <div className="grid gap-2">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground text-xs">String completa</span>
            <div className="flex gap-1">
              <Button
                size="sm"
                variant="ghost"
                disabled={rotate.isPending}
                onClick={() => rotate.mutate()}
                title="Gerar uma nova senha pro superuser (invalida a atual)"
              >
                <RefreshCw className="size-3.5" />
                {rotate.isPending ? "Regenerando..." : "Regenerar senha"}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setReveal((r) => !r)}
                title={reveal ? "Ocultar senha" : "Revelar senha"}
              >
                {reveal ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
                {reveal ? "Ocultar" : "Revelar senha"}
              </Button>
            </div>
          </div>
          <div className="bg-muted flex items-center gap-2 rounded-md border p-2">
            <code className="flex-1 overflow-x-auto font-mono text-xs whitespace-nowrap">
              {isLoading ? "Carregando..." : reveal ? connectionString : masked}
            </code>
            <Button size="icon" variant="ghost" onClick={copy} disabled={isLoading} title="Copiar">
              <Copy className="size-4" />
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="font-mono text-sm">{value}</p>
    </div>
  );
}
