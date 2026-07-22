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
import { masterApi } from "@/lib/master-api";
import { Plus } from "lucide-react";

const emptyForm = { name: "", tunnel_hostname: "", integration_key: "" };

// Cadastra uma instalação gest-postgres no mestre — hostname do túnel +
// chave de integração vêm do PRÓPRIO droplet (aba Configuração > sistema
// mestre, ainda não construída na UI local — por enquanto, valor colado à
// mão). Sem um droplet real com cloudflared configurado apontando pra cá, o
// card aparece na lista mas nunca conecta de verdade (clicar entra no
// dashboard vazio/com erro) — serve pra ver a UI, não pra uso real ainda.
export function CreateInstallationDialog() {
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: () => masterApi.createServer(form),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["master-servers"] });
      toast.success(`Instalação "${form.name}" cadastrada`);
      setForm(emptyForm);
      setOpen(false);
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : "Falha ao cadastrar instalação");
    },
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim() || !form.tunnel_hostname.trim() || !form.integration_key.trim()) {
      toast.error("Preenche todos os campos");
      return;
    }
    mutation.mutate();
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button />}>
        <Plus className="size-4" />
        Cadastrar instalação
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Cadastrar instalação gest-postgres</DialogTitle>
            <DialogDescription>
              Hostname do túnel e chave de integração vêm do próprio droplet —
              habilita o túnel Cloudflare lá (setup.sh --cloud-token) e cola os
              valores aqui.
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
            <div className="grid gap-2">
              <Label htmlFor="inst-key">Chave de integração</Label>
              <Input
                id="inst-key"
                placeholder="gpgik_..."
                value={form.integration_key}
                onChange={(e) => setForm({ ...form, integration_key: e.target.value })}
                required
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Cadastrando..." : "Cadastrar"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
