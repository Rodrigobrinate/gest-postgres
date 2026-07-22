"use client";

import { useState } from "react";
import type React from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { masterApi, type CreateMasterServerResult } from "@/lib/master-api";
import { Plus, Copy } from "lucide-react";

const emptyForm = { name: "", tunnel_hostname: "" };

// Cadastra uma instalação gest-postgres no mestre. A chave de integração é
// GERADA AQUI (o mestre) — o usuário nunca digita uma, só leva o valor
// pronto (junto com o comando completo) pro droplet. Hostname do túnel
// ainda é manual (criar o Cloudflare Tunnel em si é um passo separado, fora
// do que essa tela automatiza por enquanto).
export function CreateInstallationDialog() {
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [result, setResult] = useState<CreateMasterServerResult | null>(null);
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: () => masterApi.createServer(form),
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: ["master-servers"] });
      setResult(created);
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : "Falha ao cadastrar instalação");
    },
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim() || !form.tunnel_hostname.trim()) {
      toast.error("Preenche todos os campos");
      return;
    }
    mutation.mutate();
  }

  function close() {
    setOpen(false);
    setForm(emptyForm);
    setResult(null);
  }

  const setupCommand = result
    ? `sudo ./setup.sh --cloud-token <TOKEN_DO_TUNEL> --integration-key ${result.integration_key}`
    : "";

  function copyCommand() {
    navigator.clipboard.writeText(setupCommand);
    toast.success("Comando copiado");
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) close();
        else setOpen(true);
      }}
    >
      <DialogTrigger render={<Button />}>
        <Plus className="size-4" />
        Cadastrar instalação
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        {result ? (
          <>
            <DialogHeader>
              <DialogTitle>Instalação cadastrada</DialogTitle>
              <DialogDescription>
                Roda esse comando no droplet — a chave só aparece aqui, uma vez.
                Ainda falta trocar <code>&lt;TOKEN_DO_TUNEL&gt;</code> pelo token
                do Cloudflare Tunnel (Zero Trust &gt; Tunnels, criar um novo).
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-2 py-4">
              <Label>Comando</Label>
              <div className="flex items-center gap-2">
                <code className="bg-muted flex-1 overflow-x-auto rounded-md px-3 py-2 text-xs">
                  {setupCommand}
                </code>
                <Button type="button" variant="outline" size="icon" onClick={copyCommand}>
                  <Copy className="size-4" />
                </Button>
              </div>
            </div>
            <DialogFooter>
              <Button onClick={close}>Fechar</Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Cadastrar instalação gest-postgres</DialogTitle>
              <DialogDescription>
                A chave de integração é gerada agora, por você — não precisa ter
                uma de antes. Hostname do túnel vem de um Cloudflare Tunnel já
                criado (Zero Trust &gt; Tunnels) apontando pra esse droplet.
              </DialogDescription>
            </DialogHeader>

            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="inst-name">Nome</Label>
                <Input
                  id="inst-name"
                  placeholder="ex: droplet-producao"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  required
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="inst-hostname">Hostname do túnel</Label>
                <Input
                  id="inst-hostname"
                  placeholder="ex: droplet1.meudominio.com"
                  value={form.tunnel_hostname}
                  onChange={(e) => setForm({ ...form, tunnel_hostname: e.target.value })}
                  required
                />
              </div>
            </div>

            <DialogFooter>
              <Button type="submit" disabled={mutation.isPending}>
                {mutation.isPending ? "Cadastrando..." : "Cadastrar e gerar chave"}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
