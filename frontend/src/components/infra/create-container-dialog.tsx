"use client";

import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  api,
  ApiError,
  type BuildResult,
  type GitCredentialKind,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { GitCredentialsManager } from "@/components/infra/git-credentials-manager";
import { Plus, Trash2 } from "lucide-react";

type EnvRow = { key: string; value: string };
type PortRow = { containerPort: string; protocol: "tcp" | "udp"; hostPort: string };

function envRowsToRecord(rows: EnvRow[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const r of rows) {
    if (r.key.trim()) out[r.key.trim()] = r.value;
  }
  return out;
}

function portRowsToRecord(rows: PortRow[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const r of rows) {
    if (!r.containerPort.trim()) continue;
    out[`${r.containerPort.trim()}/${r.protocol}`] = r.hostPort.trim() ? Number(r.hostPort) : 0;
  }
  return out;
}

function EnvVarsEditor({ rows, setRows }: { rows: EnvRow[]; setRows: (r: EnvRow[]) => void }) {
  return (
    <div className="grid gap-1.5">
      <Label>Variáveis de ambiente</Label>
      <div className="grid gap-1.5">
        {rows.map((row, i) => (
          <div key={i} className="flex items-center gap-1.5">
            <Input
              value={row.key}
              onChange={(e) => setRows(rows.map((r, j) => (j === i ? { ...r, key: e.target.value } : r)))}
              placeholder="CHAVE"
              className="font-mono text-xs"
            />
            <Input
              value={row.value}
              onChange={(e) => setRows(rows.map((r, j) => (j === i ? { ...r, value: e.target.value } : r)))}
              placeholder="valor"
              className="font-mono text-xs"
            />
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              className="text-red-600"
              onClick={() => setRows(rows.filter((_, j) => j !== i))}
            >
              <Trash2 className="size-3.5" />
            </Button>
          </div>
        ))}
      </div>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="justify-self-start"
        onClick={() => setRows([...rows, { key: "", value: "" }])}
      >
        <Plus className="size-3.5" />
        Variável
      </Button>
    </div>
  );
}

function PortMappingsEditor({ rows, setRows }: { rows: PortRow[]; setRows: (r: PortRow[]) => void }) {
  return (
    <div className="grid gap-1.5">
      <Label>Portas</Label>
      <div className="grid gap-1.5">
        {rows.map((row, i) => (
          <div key={i} className="flex items-center gap-1.5">
            <Input
              value={row.containerPort}
              onChange={(e) =>
                setRows(rows.map((r, j) => (j === i ? { ...r, containerPort: e.target.value } : r)))
              }
              placeholder="porta no container (ex: 80)"
              className="font-mono text-xs"
            />
            <Select
              value={row.protocol}
              onValueChange={(v) => v && setRows(rows.map((r, j) => (j === i ? { ...r, protocol: v as "tcp" | "udp" } : r)))}
            >
              <SelectTrigger className="w-20 shrink-0">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="tcp">tcp</SelectItem>
                <SelectItem value="udp">udp</SelectItem>
              </SelectContent>
            </Select>
            <Input
              value={row.hostPort}
              onChange={(e) => setRows(rows.map((r, j) => (j === i ? { ...r, hostPort: e.target.value } : r)))}
              placeholder="porta no host (opcional)"
              className="font-mono text-xs"
            />
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              className="text-red-600"
              onClick={() => setRows(rows.filter((_, j) => j !== i))}
            >
              <Trash2 className="size-3.5" />
            </Button>
          </div>
        ))}
      </div>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="justify-self-start"
        onClick={() => setRows([...rows, { containerPort: "", protocol: "tcp", hostPort: "" }])}
      >
        <Plus className="size-3.5" />
        Porta
      </Button>
    </div>
  );
}

const DOCKERFILE_PLACEHOLDER = `FROM alpine:3.21
RUN echo "hello" > /hello.txt
CMD ["cat", "/hello.txt"]
`;

const COMPOSE_PLACEHOLDER = `services:
  web:
    image: nginx:alpine
    ports:
      - "8081:80"
`;

export function CreateContainerDialog({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<"image" | "dockerfile" | "compose" | "git">("image");

  // Modo Imagem
  const [imgName, setImgName] = useState("");
  const [image, setImage] = useState("");
  const [imgEnv, setImgEnv] = useState<EnvRow[]>([]);
  const [imgPorts, setImgPorts] = useState<PortRow[]>([]);

  const createImage = useMutation({
    mutationFn: () =>
      api.createInfraContainer({
        name: imgName,
        image,
        env: envRowsToRecord(imgEnv),
        ports: portRowsToRecord(imgPorts),
      }),
    onSuccess: () => {
      toast.success("Container criado");
      close();
      onCreated();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar container"),
  });

  // Modo Dockerfile
  const [dfName, setDfName] = useState("");
  const [dfTag, setDfTag] = useState("");
  const [dockerfile, setDockerfile] = useState(DOCKERFILE_PLACEHOLDER);
  const [dfFile, setDfFile] = useState<File | null>(null);
  const [dfContextMode, setDfContextMode] = useState<"dockerfile" | "context">("dockerfile");
  const [dfEnv, setDfEnv] = useState<EnvRow[]>([]);
  const [dfPorts, setDfPorts] = useState<PortRow[]>([]);
  const [dfLog, setDfLog] = useState<string | null>(null);

  const createFromDockerfile = useMutation({
    mutationFn: async () => {
      const result: BuildResult =
        dfContextMode === "dockerfile"
          ? await api.buildFromDockerfile(dfTag, dockerfile)
          : await api.buildFromContext(dfTag, dfFile as File);
      if (!result.success) {
        setDfLog(result.log);
        throw new Error("Build falhou — veja o log abaixo");
      }
      return api.createInfraContainer({
        name: dfName,
        image: dfTag,
        env: envRowsToRecord(dfEnv),
        ports: portRowsToRecord(dfPorts),
      });
    },
    onSuccess: () => {
      toast.success("Imagem buildada e container criado");
      close();
      onCreated();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : e instanceof Error ? e.message : "Falha ao buildar"),
  });

  // Modo Compose
  const [composeName, setComposeName] = useState("");
  const [composeContent, setComposeContent] = useState(COMPOSE_PLACEHOLDER);
  const [composeLog, setComposeLog] = useState<string | null>(null);

  const deployCompose = useMutation({
    mutationFn: () => api.deployCompose(composeName, composeContent),
    onSuccess: (result) => {
      if (result.status === "deployed") {
        toast.success(`Stack "${result.name}" implantado`);
        close();
        onCreated();
      } else {
        setComposeLog(result.last_error ?? "Falha desconhecida");
      }
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao implantar"),
  });

  // Modo Git
  const [gitName, setGitName] = useState("");
  const [gitTag, setGitTag] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [branch, setBranch] = useState("main");
  const [credentialId, setCredentialId] = useState<string>("");
  const [gitEnv, setGitEnv] = useState<EnvRow[]>([]);
  const [gitPorts, setGitPorts] = useState<PortRow[]>([]);
  const [gitLog, setGitLog] = useState<string | null>(null);

  const { data: credentials } = useQuery({
    queryKey: ["git-credentials"],
    queryFn: () => api.listGitCredentials(),
    enabled: open && mode === "git",
  });

  const createFromGit = useMutation({
    mutationFn: () =>
      api.createContainerFromGit({
        name: gitName,
        tag: gitTag,
        repo_url: repoUrl,
        branch,
        credential_id: credentialId || undefined,
        env: envRowsToRecord(gitEnv),
        ports: portRowsToRecord(gitPorts),
      }),
    onSuccess: (result) => {
      if (result.id) {
        toast.success("Repositório clonado, buildado e container criado");
        close();
        onCreated();
      } else {
        setGitLog(result.build?.log ?? "Falha desconhecida");
      }
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar a partir do Git"),
  });

  function close() {
    setOpen(false);
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus className="size-4" />
        Novo container
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Novo container</DialogTitle>
        </DialogHeader>

        <Tabs value={mode} onValueChange={(v) => v && setMode(v as typeof mode)}>
          <TabsList>
            <TabsTrigger value="image">Imagem</TabsTrigger>
            <TabsTrigger value="dockerfile">Dockerfile</TabsTrigger>
            <TabsTrigger value="compose">Compose</TabsTrigger>
            <TabsTrigger value="git">Git</TabsTrigger>
          </TabsList>

          <TabsContent value="image" className="grid gap-3 pt-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label>Nome</Label>
                <Input value={imgName} onChange={(e) => setImgName(e.target.value)} placeholder="meu-container" />
              </div>
              <div className="grid gap-1.5">
                <Label>Imagem</Label>
                <Input value={image} onChange={(e) => setImage(e.target.value)} placeholder="nginx:alpine" />
              </div>
            </div>
            <EnvVarsEditor rows={imgEnv} setRows={setImgEnv} />
            <PortMappingsEditor rows={imgPorts} setRows={setImgPorts} />
            <DialogFooter>
              <Button
                disabled={createImage.isPending || !imgName.trim() || !image.trim()}
                onClick={() => createImage.mutate()}
              >
                {createImage.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </TabsContent>

          <TabsContent value="dockerfile" className="grid gap-3 pt-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label>Nome do container</Label>
                <Input value={dfName} onChange={(e) => setDfName(e.target.value)} placeholder="meu-container" />
              </div>
              <div className="grid gap-1.5">
                <Label>Tag da imagem</Label>
                <Input value={dfTag} onChange={(e) => setDfTag(e.target.value)} placeholder="minha-imagem:latest" />
              </div>
            </div>

            <div className="flex items-center gap-4">
              <label className="flex items-center gap-1.5 text-sm">
                <input
                  type="radio"
                  checked={dfContextMode === "dockerfile"}
                  onChange={() => setDfContextMode("dockerfile")}
                />
                Só Dockerfile
              </label>
              <label className="flex items-center gap-1.5 text-sm">
                <input
                  type="radio"
                  checked={dfContextMode === "context"}
                  onChange={() => setDfContextMode("context")}
                />
                Contexto completo (.tar / .tar.gz)
              </label>
            </div>

            {dfContextMode === "dockerfile" ? (
              <div className="grid gap-1.5">
                <Label>Dockerfile</Label>
                <textarea
                  value={dockerfile}
                  onChange={(e) => setDockerfile(e.target.value)}
                  className="h-40 resize-none rounded-md border bg-background p-2 font-mono text-xs"
                  spellCheck={false}
                />
              </div>
            ) : (
              <div className="grid gap-1.5">
                <Label>Arquivo (.tar ou .tar.gz com Dockerfile na raiz)</Label>
                <input
                  type="file"
                  accept=".tar,.tar.gz,.tgz"
                  onChange={(e) => setDfFile(e.target.files?.[0] ?? null)}
                  className="text-sm"
                />
              </div>
            )}

            <EnvVarsEditor rows={dfEnv} setRows={setDfEnv} />
            <PortMappingsEditor rows={dfPorts} setRows={setDfPorts} />

            {dfLog && (
              <pre className="bg-red-50 text-red-800 max-h-40 overflow-auto rounded-md p-2 text-xs">{dfLog}</pre>
            )}

            <DialogFooter>
              <Button
                disabled={
                  createFromDockerfile.isPending ||
                  !dfName.trim() ||
                  !dfTag.trim() ||
                  (dfContextMode === "dockerfile" ? !dockerfile.trim() : !dfFile)
                }
                onClick={() => {
                  setDfLog(null);
                  createFromDockerfile.mutate();
                }}
              >
                {createFromDockerfile.isPending ? "Buildando..." : "Buildar e criar"}
              </Button>
            </DialogFooter>
          </TabsContent>

          <TabsContent value="compose" className="grid gap-3 pt-3">
            <div className="grid gap-1.5">
              <Label>Nome do stack</Label>
              <Input value={composeName} onChange={(e) => setComposeName(e.target.value)} placeholder="meu-stack" />
            </div>
            <div className="grid gap-1.5">
              <Label>docker-compose.yml</Label>
              <textarea
                value={composeContent}
                onChange={(e) => setComposeContent(e.target.value)}
                className="h-52 resize-none rounded-md border bg-background p-2 font-mono text-xs"
                spellCheck={false}
              />
            </div>
            {composeLog && (
              <pre className="bg-red-50 text-red-800 max-h-40 overflow-auto rounded-md p-2 text-xs">{composeLog}</pre>
            )}
            <DialogFooter>
              <Button
                disabled={deployCompose.isPending || !composeName.trim() || !composeContent.trim()}
                onClick={() => {
                  setComposeLog(null);
                  deployCompose.mutate();
                }}
              >
                {deployCompose.isPending ? "Implantando..." : "Implantar"}
              </Button>
            </DialogFooter>
          </TabsContent>

          <TabsContent value="git" className="grid gap-3 pt-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label>Nome do container</Label>
                <Input value={gitName} onChange={(e) => setGitName(e.target.value)} placeholder="meu-container" />
              </div>
              <div className="grid gap-1.5">
                <Label>Tag da imagem</Label>
                <Input value={gitTag} onChange={(e) => setGitTag(e.target.value)} placeholder="minha-imagem:latest" />
              </div>
            </div>
            <div className="grid grid-cols-[1fr_auto] gap-3">
              <div className="grid gap-1.5">
                <Label>URL do repositório</Label>
                <Input
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  placeholder="https://github.com/org/repo.git ou git@github.com:org/repo.git"
                />
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
                      {c.name} ({credentialKindLabel(c.kind)})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <EnvVarsEditor rows={gitEnv} setRows={setGitEnv} />
            <PortMappingsEditor rows={gitPorts} setRows={setGitPorts} />
            {gitLog && (
              <pre className="bg-red-50 text-red-800 max-h-40 overflow-auto rounded-md p-2 text-xs">{gitLog}</pre>
            )}
            <DialogFooter>
              <Button
                disabled={createFromGit.isPending || !gitName.trim() || !gitTag.trim() || !repoUrl.trim()}
                onClick={() => {
                  setGitLog(null);
                  createFromGit.mutate();
                }}
              >
                {createFromGit.isPending ? "Clonando e buildando..." : "Clonar e criar"}
              </Button>
            </DialogFooter>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

function credentialKindLabel(kind: GitCredentialKind) {
  return kind === "ssh_key" ? "chave SSH" : "PAT";
}
