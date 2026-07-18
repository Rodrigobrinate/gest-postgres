"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type ComposeProject } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Layers, Pencil, Plus, Trash2 } from "lucide-react";

const PLACEHOLDER = `services:
  web:
    image: nginx:alpine
    ports:
      - "8081:80"
`;

export function ComposeTab() {
  const { data: projects, isLoading } = useQuery({
    queryKey: ["infra-compose"],
    queryFn: () => api.listComposeProjects(),
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["infra-compose"] });

  const [editing, setEditing] = useState<ComposeProject | null>(null);
  const [creating, setCreating] = useState(false);

  const remove = useMutation({
    mutationFn: (name: string) => api.removeComposeProject(name, false),
    onSuccess: () => {
      toast.success("Stack removido");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover stack"),
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <Layers className="size-4" />
          Stacks (docker-compose)
        </CardTitle>
        <Button size="sm" onClick={() => setCreating(true)}>
          <Plus className="size-4" />
          Novo stack
        </Button>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !projects || projects.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhum stack implantado.</p>
        ) : (
          <ul className="divide-y">
            {projects.map((p) => (
              <li key={p.id} className="flex items-start justify-between gap-3 p-3 text-sm">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-mono font-medium">{p.name}</span>
                    {p.status === "deployed" ? (
                      <Badge className="bg-emerald-600 text-white">deployed</Badge>
                    ) : (
                      <Badge className="bg-red-600 text-white">erro</Badge>
                    )}
                  </div>
                  {p.last_error && (
                    <p className="text-red-600 mt-1 max-w-xl truncate text-xs" title={p.last_error}>
                      {p.last_error}
                    </p>
                  )}
                </div>
                <div className="flex shrink-0 gap-1">
                  <Button size="icon" variant="ghost" title="Editar/redeploy" onClick={() => setEditing(p)}>
                    <Pencil className="size-4" />
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="text-red-600"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(p.name)}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>

      {(creating || editing) && (
        <DeployDialog
          project={editing}
          onClose={() => {
            setCreating(false);
            setEditing(null);
          }}
          onDeployed={() => {
            setCreating(false);
            setEditing(null);
            invalidate();
          }}
        />
      )}
    </Card>
  );
}

function DeployDialog({
  project,
  onClose,
  onDeployed,
}: {
  project: ComposeProject | null;
  onClose: () => void;
  onDeployed: () => void;
}) {
  const [name, setName] = useState(project?.name ?? "");
  const [content, setContent] = useState(project?.compose_content ?? PLACEHOLDER);
  const [log, setLog] = useState<string | null>(null);

  const deploy = useMutation({
    mutationFn: () => api.deployCompose(name, content),
    onSuccess: (result) => {
      if (result.status === "deployed") {
        toast.success(`Stack "${result.name}" implantado`);
        onDeployed();
      } else {
        setLog(result.last_error ?? "Falha desconhecida");
      }
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao implantar"),
  });

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{project ? `Editar "${project.name}"` : "Novo stack"}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-3 py-2">
          <div className="grid gap-1.5">
            <Label>Nome do stack</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!!project}
              placeholder="meu-stack"
            />
          </div>
          <div className="grid gap-1.5">
            <Label>docker-compose.yml</Label>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              className="h-64 resize-none rounded-md border bg-background p-2 font-mono text-xs"
              spellCheck={false}
            />
          </div>
          {log && (
            <pre className="bg-muted max-h-40 overflow-auto rounded-md p-2 text-xs text-red-600">{log}</pre>
          )}
        </div>
        <DialogFooter>
          <Button disabled={deploy.isPending || !name.trim() || !content.trim()} onClick={() => deploy.mutate()}>
            {deploy.isPending ? "Implantando..." : "Implantar"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
