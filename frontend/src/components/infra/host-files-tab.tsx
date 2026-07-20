"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { FileBrowser, type FileBrowserAdapter } from "@/components/infra/file-browser";
import { StepUpDialog } from "@/components/auth/step-up-dialog";
import { HardDrive, ShieldAlert, ShieldCheck } from "lucide-react";

// Elevação dura 5 minutos no backend (ver internal/auth StepUp) — replica
// isso no front só pra não deixar os botões de escrita/exclusão acesos
// depois que a sessão já perdeu a elevação de verdade; se o usuário tentar
// mesmo assim, a API responde 403 normalmente.
const ELEVATION_MS = 5 * 60 * 1000;

export function HostFilesTab() {
  const [elevated, setElevated] = useState(false);
  const [stepUpOpen, setStepUpOpen] = useState(false);

  useEffect(() => {
    if (!elevated) return;
    const timer = setTimeout(() => setElevated(false), ELEVATION_MS);
    return () => clearTimeout(timer);
  }, [elevated]);

  const adapter: FileBrowserAdapter = {
    list: (path) => api.listHostFiles(path),
    stat: (path) => api.statHostFile(path),
    read: (path) => api.readHostFile(path),
    write: (path, content) => api.writeHostFile(path, content),
    upload: (path, file) => api.uploadHostFile(path, file),
    remove: (path) => api.deleteHostFile(path),
    downloadUrl: (path) => api.hostFileDownloadUrl(path),
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <HardDrive className="size-4" />
          Arquivos do host
        </CardTitle>
        {elevated ? (
          <span className="text-emerald-600 flex items-center gap-1.5 text-xs font-medium">
            <ShieldCheck className="size-4" />
            Zona de risco destravada
          </span>
        ) : (
          <Button size="sm" variant="outline" onClick={() => setStepUpOpen(true)}>
            <ShieldAlert className="size-4" />
            Destravar edição/exclusão
          </Button>
        )}
      </CardHeader>
      <CardContent>
        <p className="text-muted-foreground mb-3 text-xs">
          Navega numa pasta fixa do servidor (não o filesystem inteiro) — configurada em
          <code className="mx-1">HOST_FILES_ROOT</code>. Listar/ler/baixar não precisam de
          confirmação extra; editar, enviar e excluir exigem confirmar a senha de novo.
        </p>
        <FileBrowser adapter={adapter} queryKeyPrefix="host-files" readOnly={!elevated} />
      </CardContent>

      {stepUpOpen && (
        <StepUpDialog
          onClose={() => setStepUpOpen(false)}
          onElevated={() => {
            setElevated(true);
            setStepUpOpen(false);
          }}
        />
      )}
    </Card>
  );
}
