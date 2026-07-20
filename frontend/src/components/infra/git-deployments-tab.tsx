"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, API_URL } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { GitCredentialsManager } from "@/components/infra/git-credentials-manager";
import {
  EnvVarsEditor,
  PortMappingsEditor,
  envRowsToRecord,
  portRowsToRecord,
  type EnvRow,
  type PortRow,
} from "@/components/infra/create-container-dialog";
import { GitBranch, Plus, RotateCw, Trash2, Webhook } from "lucide-react";

export function GitDeploymentsTab() {
  const { data: deployments, isLoading } = useQuery({
    queryKey: ["git-deployments"],
    queryFn: () => api.listGitDeployments(),
    refetchInterval: 15_000,
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["git-deployments"] });

  const [open, setOpen] = useState(false);

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteGitDeployment(id),
    onSuccess: () => {
      toast.success("Deploy automático removido (container continua no ar)");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover"),
  });

  const redeploy = useMutation({
    mutationFn: (id: string) => api.redeployGitDeploymentNow(id),
    onSuccess: () => {
      toast.success("Reimplantação concluída");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao reimplantar"),
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <Webhook className="size-4" />
          Deploy automático (Git)
        </CardTitle>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" />}>
            <Plus className="size-4" />
            Novo
          </DialogTrigger>
          <CreateGitDeploymentDialog onClose={() => setOpen(false)} onCreated={invalidate} />
        </Dialog>
      </CardHeader>
      <CardContent className="p-0">
        <p className="text-muted-foreground px-4 pb-3 text-xs">
          Guarda a config de clone+build de um repositório e reimplanta o container sozinho a cada
          push, via webhook (GitHub/GitLab chamam essa URL). Diferente do modo Git do &ldquo;Novo
          container&rdquo;, que é um disparo único, sem acompanhar push nenhum depois.
        </p>
        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !deployments || deployments.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhum deploy automático configurado.</p>
        ) : (
          <ul className="divide-y">
            {deployments.map((d) => (
              <li key={d.id} className="flex flex-col gap-1 px-4 py-3 text-sm">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <GitBranch className="text-muted-foreground size-4" />
                    <span className="font-mono font-medium">{d.container_name}</span>
                    <Badge variant="outline">{d.branch}</Badge>
                    {d.last_status === "success" && <Badge className="bg-emerald-600 text-white">ok</Badge>}
                    {d.last_status === "failed" && <Badge className="bg-red-600 text-white">falhou</Badge>}
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      size="icon-xs"
                      variant="ghost"
                      title="Reimplantar agora"
                      disabled={redeploy.isPending}
                      onClick={() => redeploy.mutate(d.id)}
                    >
                      <RotateCw className="size-3.5" />
                    </Button>
                    <Button
                      size="icon-xs"
                      variant="ghost"
                      className="text-red-600"
                      title="Remover (não apaga o container)"
                      disabled={remove.isPending}
                      onClick={() => {
                        if (confirm(`Parar de acompanhar "${d.container_name}"? O container continua no ar.`)) {
                          remove.mutate(d.id);
                        }
                      }}
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                </div>
                <span className="text-muted-foreground font-mono text-xs">{d.repo_url}</span>
                {d.last_deployed_at && (
                  <span className="text-muted-foreground text-xs">
                    último deploy: {new Date(d.last_deployed_at).toLocaleString("pt-BR")}
                    {d.last_status === "failed" && d.last_error ? ` — ${d.last_error}` : ""}
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

function CreateGitDeploymentDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [containerName, setContainerName] = useState("");
  const [imageTag, setImageTag] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [branch, setBranch] = useState("main");
  const [credentialId, setCredentialId] = useState("");
  const [env, setEnv] = useState<EnvRow[]>([]);
  const [ports, setPorts] = useState<PortRow[]>([]);
  const [webhookInfo, setWebhookInfo] = useState<{ url: string; secret: string } | null>(null);

  const { data: credentials } = useQuery({
    queryKey: ["git-credentials"],
    queryFn: () => api.listGitCredentials(),
  });

  const create = useMutation({
    mutationFn: () =>
      api.createGitDeployment({
        container_name: containerName,
        image_tag: imageTag,
        repo_url: repoUrl,
        branch,
        credential_id: credentialId || undefined,
        env: envRowsToRecord(env),
        ports: portRowsToRecord(ports),
      }),
    onSuccess: (result) => {
      onCreated();
      if (result.deployment.last_status === "success") {
        toast.success("Deploy automático criado e primeiro deploy concluído");
      } else {
        toast.error(`Deployment salvo, mas o primeiro build falhou: ${result.deployment.last_error ?? ""}`);
      }
      setWebhookInfo({ url: `${API_URL}${result.webhook_url_path}`, secret: result.webhook_secret });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar deploy automático"),
  });

  if (webhookInfo) {
    return (
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Configure o webhook no GitHub/GitLab</DialogTitle>
        </DialogHeader>
        <div className="grid gap-3 text-sm">
          <p className="text-muted-foreground text-xs">
            Esse segredo só aparece uma vez — cole os dois agora nas configurações de webhook do
            repositório (Settings → Webhooks). Se perder, exclui e cria de novo.
          </p>
          <div className="grid gap-1.5">
            <Label>Payload URL</Label>
            <Input readOnly value={webhookInfo.url} className="font-mono text-xs" onFocus={(e) => e.target.select()} />
          </div>
          <div className="grid gap-1.5">
            <Label>Secret</Label>
            <Input
              readOnly
              value={webhookInfo.secret}
              className="font-mono text-xs"
              onFocus={(e) => e.target.select()}
            />
          </div>
          <p className="text-muted-foreground text-xs">
            GitHub: Content type <code>application/json</code>, evento só &ldquo;push&rdquo;. GitLab: cola
            o secret no campo &ldquo;Secret Token&rdquo; e habilita &ldquo;Push events&rdquo;.
          </p>
        </div>
        <DialogFooter>
          <Button onClick={onClose}>Concluído</Button>
        </DialogFooter>
      </DialogContent>
    );
  }

  return (
    <DialogContent className="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>Novo deploy automático</DialogTitle>
      </DialogHeader>
      <div className="grid gap-3 py-2">
        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-1.5">
            <Label>Nome do container</Label>
            <Input value={containerName} onChange={(e) => setContainerName(e.target.value)} />
          </div>
          <div className="grid gap-1.5">
            <Label>Tag da imagem</Label>
            <Input value={imageTag} onChange={(e) => setImageTag(e.target.value)} placeholder="minha-app:latest" />
          </div>
        </div>
        <div className="grid grid-cols-[1fr_auto] gap-3">
          <div className="grid gap-1.5">
            <Label>URL do repositório</Label>
            <Input value={repoUrl} onChange={(e) => setRepoUrl(e.target.value)} />
          </div>
          <div className="grid gap-1.5">
            <Label>Branch</Label>
            <Input value={branch} onChange={(e) => setBranch(e.target.value)} className="w-28" />
          </div>
        </div>
        <div className="grid gap-1.5">
          <div className="flex items-center justify-between">
            <Label>Credencial (repositório privado)</Label>
            <GitCredentialsManager />
          </div>
          <Select value={credentialId} onValueChange={(v) => setCredentialId(v ?? "")}>
            <SelectTrigger>
              <SelectValue placeholder="Nenhuma (repositório público)" />
            </SelectTrigger>
            <SelectContent>
              {(credentials ?? []).map((c) => (
                <SelectItem key={c.id} value={c.id}>
                  {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <EnvVarsEditor rows={env} setRows={setEnv} />
        <PortMappingsEditor rows={ports} setRows={setPorts} />
      </div>
      <DialogFooter>
        <Button
          disabled={create.isPending || !containerName.trim() || !imageTag.trim() || !repoUrl.trim()}
          onClick={() => create.mutate()}
        >
          {create.isPending ? "Implantando..." : "Criar e implantar"}
        </Button>
      </DialogFooter>
    </DialogContent>
  );
}
