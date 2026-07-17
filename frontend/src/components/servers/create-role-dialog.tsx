"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
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
import { Plus } from "lucide-react";

const emptyForm = {
  name: "",
  password: "",
  can_login: true,
  superuser: false,
  create_db: false,
  create_role: false,
  connection_limit: -1,
};

export function CreateRoleDialog({ serverId }: { serverId: string }) {
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);

  const queryClient = useQueryClient();
  const create = useMutation({
    mutationFn: () => api.createRole(serverId, form),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "roles"] });
      toast.success(`Usuário "${form.name}" criado`);
      setOpen(false);
      setForm(emptyForm);
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar usuário"),
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) return toast.error("Nome é obrigatório");
    create.mutate();
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Plus className="size-4" />
        Novo usuário
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Criar usuário</DialogTitle>
            <DialogDescription>Cria uma role no cluster Postgres.</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="role-name">Nome</Label>
                <Input
                  id="role-name"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  required
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="role-password">Senha</Label>
                <Input
                  id="role-password"
                  type="text"
                  value={form.password}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                  placeholder="opcional"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-2">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.can_login}
                  onChange={(e) => setForm({ ...form, can_login: e.target.checked })}
                />
                Pode logar (LOGIN)
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.superuser}
                  onChange={(e) => setForm({ ...form, superuser: e.target.checked })}
                />
                Superusuário
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.create_db}
                  onChange={(e) => setForm({ ...form, create_db: e.target.checked })}
                />
                Pode criar banco
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.create_role}
                  onChange={(e) => setForm({ ...form, create_role: e.target.checked })}
                />
                Pode criar usuário
              </label>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="conn-limit">Limite de conexões</Label>
              <Input
                id="conn-limit"
                type="number"
                value={form.connection_limit}
                onChange={(e) => setForm({ ...form, connection_limit: Number(e.target.value) })}
              />
              <p className="text-muted-foreground text-xs">-1 = sem limite</p>
            </div>
          </div>

          <DialogFooter>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? "Criando..." : "Criar usuário"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
