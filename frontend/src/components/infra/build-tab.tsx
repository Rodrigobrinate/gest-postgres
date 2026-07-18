"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type BuildResult } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Hammer } from "lucide-react";

const PLACEHOLDER = `FROM alpine:3.21
RUN echo "hello" > /hello.txt
CMD ["cat", "/hello.txt"]
`;

export function BuildTab() {
  const [mode, setMode] = useState<"dockerfile" | "context">("dockerfile");
  const [tag, setTag] = useState("");
  const [dockerfile, setDockerfile] = useState(PLACEHOLDER);
  const [file, setFile] = useState<File | null>(null);

  const build = useMutation({
    mutationFn: () =>
      mode === "dockerfile"
        ? api.buildFromDockerfile(tag, dockerfile)
        : api.buildFromContext(tag, file as File),
    onSuccess: (result: BuildResult) => {
      if (result.success) toast.success(`Imagem "${result.tag}" construída`);
      else toast.error("Build falhou — veja o log abaixo");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao buildar"),
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base flex items-center gap-1.5">
          <Hammer className="size-4" />
          Build via Dockerfile
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex items-center gap-4">
          <label className="flex items-center gap-1.5 text-sm">
            <input
              type="radio"
              checked={mode === "dockerfile"}
              onChange={() => setMode("dockerfile")}
            />
            Só Dockerfile
          </label>
          <label className="flex items-center gap-1.5 text-sm">
            <input type="radio" checked={mode === "context"} onChange={() => setMode("context")} />
            Contexto completo (.tar / .tar.gz)
          </label>
        </div>

        <div className="grid gap-1.5">
          <Label>Tag da imagem</Label>
          <Input value={tag} onChange={(e) => setTag(e.target.value)} placeholder="minha-imagem:latest" />
        </div>

        {mode === "dockerfile" ? (
          <div className="grid gap-1.5">
            <Label>Dockerfile</Label>
            <textarea
              value={dockerfile}
              onChange={(e) => setDockerfile(e.target.value)}
              className="h-64 resize-none rounded-md border bg-background p-2 font-mono text-xs"
              spellCheck={false}
            />
            <p className="text-muted-foreground text-xs">
              Sem contexto extra — só serve se o Dockerfile não fizer COPY de arquivo nenhum.
            </p>
          </div>
        ) : (
          <div className="grid gap-1.5">
            <Label>Arquivo (.tar ou .tar.gz com Dockerfile na raiz)</Label>
            <input
              type="file"
              accept=".tar,.tar.gz,.tgz"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              className="text-sm"
            />
          </div>
        )}

        <div>
          <Button
            disabled={
              build.isPending || !tag.trim() || (mode === "dockerfile" ? !dockerfile.trim() : !file)
            }
            onClick={() => build.mutate()}
          >
            {build.isPending ? "Buildando..." : "Buildar"}
          </Button>
        </div>

        {build.data && (
          <pre
            className={`max-h-80 overflow-auto rounded-md p-3 text-xs ${
              build.data.success ? "bg-muted" : "bg-red-50 text-red-800"
            }`}
          >
            {build.data.log}
          </pre>
        )}
      </CardContent>
    </Card>
  );
}
