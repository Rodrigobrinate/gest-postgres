"use client";

import { useState } from "react";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api, ApiError, type CreateServerInput, type Preset } from "@/lib/api";
import { Plus } from "lucide-react";

const VERSIONS = ["17", "16", "15", "14", "13"];

const PRESETS: { value: Preset; label: string; hint: string }[] = [
  { value: "small", label: "Pequeno", hint: "1 vCPU · 1 GB RAM · 10 GB disco — dev/teste" },
  { value: "medium", label: "Médio", hint: "2 vCPU · 4 GB RAM · 50 GB disco — produção pequena" },
  { value: "large", label: "Grande", hint: "4 vCPU · 16 GB RAM · 200 GB disco — produção" },
  { value: "custom", label: "Customizado", hint: "Você define CPU, RAM e disco" },
];

const emptyForm = {
  name: "",
  description: "",
  version: "16",
  preset: "small" as Preset,
  cpu_cores: 1,
  memory_mb: 1024,
  disk_gb: 10,
  username: "",
  password: "",
  database_name: "",
};

export function CreateServerDialog() {
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: (input: CreateServerInput) => api.createServer(input),
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: ["servers"] });
      toast.success(`Servidor "${created.name}" criado`, {
        description: "Provisionando container — status atualiza automaticamente.",
      });
      setForm(emptyForm);
      setOpen(false);
    },
    onError: (err) => {
      const message = err instanceof ApiError ? err.message : "Falha ao criar servidor";
      toast.error(message);
    },
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) {
      toast.error("Nome do servidor é obrigatório");
      return;
    }

    const input: CreateServerInput = {
      name: form.name.trim(),
      description: form.description.trim(),
      version: form.version,
      preset: form.preset,
      username: form.username.trim() || undefined,
      password: form.password || undefined,
      database_name: form.database_name.trim() || undefined,
    };

    if (form.preset === "custom") {
      input.resources = {
        cpu_cores: Number(form.cpu_cores),
        memory_mb: Number(form.memory_mb),
        disk_gb: Number(form.disk_gb),
      };
    }

    mutation.mutate(input);
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button />}>
        <Plus className="size-4" />
        Criar servidor
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Criar servidor PostgreSQL</DialogTitle>
            <DialogDescription>
              Configuração já vem pré-preenchida com valores recomendados. Dá pra
              editar tudo depois de criado.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="name">Nome</Label>
              <Input
                id="name"
                placeholder="ex: producao-api"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                required
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="description">Descrição (opcional)</Label>
              <Input
                id="description"
                placeholder="ex: Banco da API principal"
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label>Versão do PostgreSQL</Label>
                <Select
                  value={form.version}
                  onValueChange={(v) => v && setForm({ ...form, version: v })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {VERSIONS.map((v) => (
                      <SelectItem key={v} value={v}>
                        PostgreSQL {v}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="grid gap-2">
                <Label>Tamanho</Label>
                <Select
                  value={form.preset}
                  onValueChange={(v) => v && setForm({ ...form, preset: v as Preset })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PRESETS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <p className="text-muted-foreground -mt-2 text-xs">
              {PRESETS.find((p) => p.value === form.preset)?.hint}
            </p>

            {form.preset === "custom" && (
              <div className="grid grid-cols-3 gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="cpu">CPU (cores)</Label>
                  <Input
                    id="cpu"
                    type="number"
                    min={1}
                    step="0.5"
                    value={form.cpu_cores}
                    onChange={(e) => setForm({ ...form, cpu_cores: Number(e.target.value) })}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="mem">RAM (MB)</Label>
                  <Input
                    id="mem"
                    type="number"
                    min={256}
                    value={form.memory_mb}
                    onChange={(e) => setForm({ ...form, memory_mb: Number(e.target.value) })}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="disk">Disco (GB)</Label>
                  <Input
                    id="disk"
                    type="number"
                    min={1}
                    value={form.disk_gb}
                    onChange={(e) => setForm({ ...form, disk_gb: Number(e.target.value) })}
                  />
                </div>
              </div>
            )}

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="username">Usuário admin</Label>
                <Input
                  id="username"
                  placeholder="postgres"
                  value={form.username}
                  onChange={(e) => setForm({ ...form, username: e.target.value })}
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="database_name">Banco inicial</Label>
                <Input
                  id="database_name"
                  placeholder="app"
                  value={form.database_name}
                  onChange={(e) => setForm({ ...form, database_name: e.target.value })}
                />
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="password">Senha (opcional)</Label>
              <Input
                id="password"
                type="text"
                placeholder="Deixe em branco pra gerar uma senha forte automaticamente"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Criando..." : "Criar servidor"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
