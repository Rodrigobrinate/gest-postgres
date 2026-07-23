"use client";

import { useState } from "react";
import type React from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type ColumnMeta } from "@/lib/api";
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

// Formulário livre — campo em branco fica de fora do INSERT (a coluna usa o
// DEFAULT da tabela, ex: serial/gen_random_uuid()/now()). "NULL" força null
// explícito mesmo com o campo vazio, só aparece pra coluna que aceita NULL.
export function AddRowDialog({
  serverId,
  database,
  schema,
  table,
  columns,
}: {
  serverId: string;
  database: string;
  schema: string;
  table: string;
  columns: ColumnMeta[];
}) {
  const [open, setOpen] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [nullCols, setNullCols] = useState<Set<string>>(new Set());
  const queryClient = useQueryClient();

  function reset() {
    setValues({});
    setNullCols(new Set());
  }

  const insert = useMutation({
    mutationFn: () => {
      const payload: Record<string, unknown> = {};
      for (const col of columns) {
        if (nullCols.has(col.name)) {
          payload[col.name] = null;
        } else if ((values[col.name] ?? "") !== "") {
          payload[col.name] = values[col.name];
        }
      }
      return api.insertTableRow(serverId, database, schema, table, payload);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "tableRows", database, schema, table] });
      toast.success("Linha criada");
      setOpen(false);
      reset();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar linha"),
  });

  function toggleNull(col: string) {
    setNullCols((s) => {
      const next = new Set(s);
      if (next.has(col)) next.delete(col);
      else next.add(col);
      return next;
    });
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    insert.mutate();
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) reset();
      }}
    >
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Plus className="size-4" />
        Nova linha
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Nova linha</DialogTitle>
            <DialogDescription>
              {schema}.{table} — campo em branco usa o valor padrão da coluna, se tiver.
            </DialogDescription>
          </DialogHeader>

          <div className="grid max-h-96 gap-3 overflow-y-auto py-4">
            {columns.map((col) => (
              <div key={col.name} className="grid gap-1">
                <Label htmlFor={`add-row-${col.name}`} className="flex items-center gap-1.5 text-xs">
                  <span className="font-mono">{col.name}</span>
                  <span className="text-muted-foreground">{col.data_type}</span>
                  {col.is_primary_key && <span className="text-muted-foreground">PK</span>}
                </Label>
                <div className="flex items-center gap-2">
                  <Input
                    id={`add-row-${col.name}`}
                    value={values[col.name] ?? ""}
                    disabled={nullCols.has(col.name)}
                    placeholder={col.is_nullable ? "(padrão ou null)" : "(padrão)"}
                    onChange={(e) => setValues((v) => ({ ...v, [col.name]: e.target.value }))}
                  />
                  {col.is_nullable && (
                    <label className="text-muted-foreground flex shrink-0 items-center gap-1 text-xs">
                      <input type="checkbox" checked={nullCols.has(col.name)} onChange={() => toggleNull(col.name)} />
                      NULL
                    </label>
                  )}
                </div>
              </div>
            ))}
          </div>

          <DialogFooter>
            <Button type="submit" disabled={insert.isPending}>
              {insert.isPending ? "Criando..." : "Criar linha"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
