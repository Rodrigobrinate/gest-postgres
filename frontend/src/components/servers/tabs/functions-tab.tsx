"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import CodeMirror from "@uiw/react-codemirror";
import { sql as sqlLang } from "@codemirror/lang-sql";
import { keymap } from "@codemirror/view";
import { Prec } from "@codemirror/state";
import { api, ApiError, type FunctionInfo } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { ChevronDown, ChevronRight, Plus, Trash2 } from "lucide-react";
import { cn } from "@/lib/utils";

const TEMPLATE = `CREATE FUNCTION public.minha_funcao(x integer)
RETURNS integer
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN x * 2;
END;
$$;`;

export function FunctionsTab({ serverId, database }: { serverId: string; database: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["servers", serverId, "functions", database],
    queryFn: () => api.listFunctions(serverId, database),
    enabled: !!database,
  });
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "functions"] });

  const [open, setOpen] = useState(false);
  const [sqlText, setSqlText] = useState(TEMPLATE);
  const [expanded, setExpanded] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: () => api.createFunction(serverId, database, sqlText),
    onSuccess: () => {
      toast.success("Function/procedure criada");
      setOpen(false);
      setSqlText(TEMPLATE);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar"),
  });

  const remove = useMutation({
    mutationFn: (f: FunctionInfo) => api.dropFunction(serverId, database, f.schema, f.name, f.identity_args),
    onSuccess: (_d, f) => {
      toast.success(`${f.name} excluída`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir"),
  });

  const runKeymap = Prec.highest(
    keymap.of([
      {
        key: "Mod-Enter",
        run: () => {
          if (sqlText.trim() && !create.isPending) create.mutate();
          return true;
        },
      },
    ])
  );

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" variant="outline" />}>
            <Plus className="size-4" />
            Nova function/procedure
          </DialogTrigger>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>Criar function ou procedure</DialogTitle>
            </DialogHeader>
            <div className="py-2">
              <div className="overflow-hidden rounded-md border">
                <CodeMirror
                  value={sqlText}
                  height="280px"
                  extensions={[sqlLang(), runKeymap]}
                  onChange={(value) => setSqlText(value)}
                  basicSetup={{ lineNumbers: true, foldGutter: false }}
                  className="text-sm"
                />
              </div>
              <p className="text-muted-foreground mt-2 text-xs">
                Precisa começar com CREATE [OR REPLACE] FUNCTION/PROCEDURE · Ctrl/Cmd+Enter cria
              </p>
            </div>
            <DialogFooter>
              <Button disabled={create.isPending || !sqlText.trim()} onClick={() => create.mutate()}>
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !data || data.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">
              Nenhuma function/procedure em &ldquo;{database}&rdquo;.
            </p>
          ) : (
            <ul className="divide-y">
              {data.map((f) => {
                const key = `${f.schema}.${f.name}(${f.identity_args})`;
                const isOpen = expanded === key;
                return (
                  <li key={key}>
                    <div className="flex items-start justify-between gap-3 p-3">
                      <button
                        className="flex min-w-0 flex-1 items-start gap-2 text-left"
                        onClick={() => setExpanded(isOpen ? null : key)}
                      >
                        {isOpen ? (
                          <ChevronDown className="mt-0.5 size-4 shrink-0" />
                        ) : (
                          <ChevronRight className="mt-0.5 size-4 shrink-0" />
                        )}
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-mono text-sm font-medium">
                              {f.schema}.{f.name}({f.arguments})
                            </span>
                            <Badge variant="outline">{f.kind}</Badge>
                            <Badge variant="outline">{f.language}</Badge>
                          </div>
                          {f.return_type && (
                            <p className="text-muted-foreground text-xs">retorna {f.return_type}</p>
                          )}
                        </div>
                      </button>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="shrink-0 text-red-600"
                        onClick={() => remove.mutate(f)}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </div>
                    {isOpen && (
                      <pre
                        className={cn(
                          "bg-muted/50 overflow-x-auto p-3 font-mono text-xs whitespace-pre-wrap"
                        )}
                      >
                        {f.definition}
                      </pre>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
