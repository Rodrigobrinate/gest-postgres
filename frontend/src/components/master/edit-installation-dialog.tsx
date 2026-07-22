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
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { masterApi, type MasterServerSummary } from "@/lib/master-api";
import { Pencil } from "lucide-react";

// Edita nome/hostname de uma instalação já cadastrada. Chave de integração
// NUNCA muda por aqui — é fixa desde o cadastro, trocar exigiria coordenar
// com o droplet de novo (fora de escopo de uma edição simples).
export function EditInstallationDialog({ installation }: { installation: MasterServerSummary }) {
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ name: installation.name, tunnel_hostname: installation.tunnel_hostname });
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: () => masterApi.updateServer(installation.id, form),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["master-servers"] });
      toast.success("Instalação atualizada");
      setOpen(false);
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : "Falha ao atualizar instalação"),
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim() || !form.tunnel_hostname.trim()) {
      toast.error("Preenche todos os campos");
      return;
    }
    mutation.mutate();
  }

  function openDialog(e: React.MouseEvent) {
    e.stopPropagation();
    setForm({ name: installation.name, tunnel_hostname: installation.tunnel_hostname });
    setOpen(true);
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button variant="ghost" size="icon" className="size-6" onClick={openDialog}>
        <Pencil className="size-3.5" />
      </Button>
      <DialogContent className="sm:max-w-lg" onClick={(e) => e.stopPropagation()}>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Editar instalação</DialogTitle>
            <DialogDescription>
              Chave de integração não muda por aqui — só nome e hostname do túnel.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="edit-inst-name">Nome</Label>
              <Input
                id="edit-inst-name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit-inst-hostname">Hostname do túnel</Label>
              <Input
                id="edit-inst-hostname"
                value={form.tunnel_hostname}
                onChange={(e) => setForm({ ...form, tunnel_hostname: e.target.value })}
                required
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Salvando..." : "Salvar"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
