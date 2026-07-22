"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { EnvVarsEditor, envRowsToRecord, type EnvRow } from "@/components/infra/create-container-dialog";
import { Pencil, TriangleAlert } from "lucide-react";

function parseEnv(env: string[]): EnvRow[] {
  return env.map((line) => {
    const idx = line.indexOf("=");
    return idx >= 0 ? { key: line.slice(0, idx), value: line.slice(idx + 1) } : { key: line, value: "" };
  });
}

export function EnvTab({ env, containerId }: { env: string[]; containerId: string }) {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [rows, setRows] = useState<EnvRow[]>(() => parseEnv(env ?? []));

  const save = useMutation({
    mutationFn: () => api.updateContainerEnv(containerId, envRowsToRecord(rows)),
    onSuccess: (result) => {
      toast.success("Variáveis atualizadas — container recriado com o novo ID");
      router.push(`/infra/containers?id=${result.id}`);
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar variáveis"),
  });

  if (!editing) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">Variáveis de ambiente</CardTitle>
          <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
            <Pencil className="size-4" />
            Editar
          </Button>
        </CardHeader>
        <CardContent className="p-0">
          {!env || env.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Sem variáveis de ambiente.</p>
          ) : (
            <ul className="divide-y">
              {env.map((line) => {
                const idx = line.indexOf("=");
                const key = idx >= 0 ? line.slice(0, idx) : line;
                const value = idx >= 0 ? line.slice(idx + 1) : "";
                return (
                  <li key={line} className="flex gap-2 px-4 py-2 font-mono text-xs">
                    <span className="font-medium">{key}</span>
                    <span className="text-muted-foreground truncate">={value}</span>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Editar variáveis de ambiente</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3">
        <div className="flex items-center gap-1.5 rounded-md border border-amber-300 bg-amber-50 p-3 text-xs text-amber-800">
          <TriangleAlert className="size-3.5 shrink-0" />
          Docker não deixa trocar env var de container rodando — salvar aqui para, remove e recria o
          container (breve interrupção), com o mesmo nome/imagem/portas/redes. O ID muda.
        </div>
        <EnvVarsEditor rows={rows} setRows={setRows} />
        <div className="flex gap-2">
          <Button disabled={save.isPending} onClick={() => save.mutate()}>
            {save.isPending ? "Recriando..." : "Salvar e recriar"}
          </Button>
          <Button variant="outline" disabled={save.isPending} onClick={() => setEditing(false)}>
            Cancelar
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
