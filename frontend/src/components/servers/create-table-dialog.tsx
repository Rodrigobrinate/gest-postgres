"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  api,
  ApiError,
  COLUMN_TYPES,
  type ColumnDef,
} from "@/lib/api";
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
import { Plus, Trash2 } from "lucide-react";

function emptyColumn(): ColumnDef {
  return { name: "", type: "text", not_null: false, primary_key: false, default: "" };
}

export function CreateTableDialog({
  serverId,
  database,
}: {
  serverId: string;
  database: string;
}) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [columns, setColumns] = useState<ColumnDef[]>([
    { name: "id", type: "serial", not_null: true, primary_key: true, default: "" },
    emptyColumn(),
  ]);

  const queryClient = useQueryClient();
  const create = useMutation({
    mutationFn: () =>
      api.createTable(serverId, database, {
        schema: "public",
        name,
        columns: columns.filter((c) => c.name.trim() !== ""),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "tables"] });
      toast.success(`Tabela "${name}" criada`);
      setOpen(false);
      setName("");
      setColumns([
        { name: "id", type: "serial", not_null: true, primary_key: true, default: "" },
        emptyColumn(),
      ]);
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar tabela"),
  });

  function updateColumn(index: number, patch: Partial<ColumnDef>) {
    setColumns((cols) => cols.map((c, i) => (i === index ? { ...c, ...patch } : c)));
  }

  function removeColumn(index: number) {
    setColumns((cols) => cols.filter((_, i) => i !== index));
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      toast.error("Nome da tabela é obrigatório");
      return;
    }
    if (columns.filter((c) => c.name.trim() !== "").length === 0) {
      toast.error("Adiciona pelo menos uma coluna");
      return;
    }
    create.mutate();
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Plus className="size-4" />
        Nova tabela
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Criar tabela</DialogTitle>
            <DialogDescription>Banco: {database}</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="table-name">Nome da tabela</Label>
              <Input
                id="table-name"
                placeholder="ex: pedidos"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>

            <div className="grid gap-2">
              <Label>Colunas</Label>
              <div className="grid gap-2">
                <div className="text-muted-foreground grid grid-cols-[1fr_140px_60px_60px_1fr_auto] gap-2 px-1 text-xs">
                  <span>Nome</span>
                  <span>Tipo</span>
                  <span className="text-center">Not null</span>
                  <span className="text-center">PK</span>
                  <span>Default (opcional)</span>
                  <span />
                </div>
                {columns.map((col, i) => (
                  <div
                    key={i}
                    className="grid grid-cols-[1fr_140px_60px_60px_1fr_auto] items-center gap-2"
                  >
                    <Input
                      placeholder="coluna"
                      value={col.name}
                      onChange={(e) => updateColumn(i, { name: e.target.value })}
                    />
                    <Select
                      value={col.type}
                      onValueChange={(v) => v && updateColumn(i, { type: v })}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {COLUMN_TYPES.map((t) => (
                          <SelectItem key={t} value={t}>
                            {t}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <input
                      type="checkbox"
                      className="mx-auto"
                      checked={col.not_null}
                      onChange={(e) => updateColumn(i, { not_null: e.target.checked })}
                    />
                    <input
                      type="checkbox"
                      className="mx-auto"
                      checked={col.primary_key}
                      onChange={(e) => updateColumn(i, { primary_key: e.target.checked })}
                    />
                    <Input
                      placeholder="ex: now()"
                      value={col.default}
                      onChange={(e) => updateColumn(i, { default: e.target.value })}
                    />
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      className="text-red-600"
                      onClick={() => removeColumn(i)}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                ))}
              </div>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="w-fit"
                onClick={() => setColumns((cols) => [...cols, emptyColumn()])}
              >
                <Plus className="size-3.5" />
                Adicionar coluna
              </Button>
            </div>
          </div>

          <DialogFooter>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? "Criando..." : "Criar tabela"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
