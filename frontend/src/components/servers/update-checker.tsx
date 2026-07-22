"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { StepUpDialog } from "@/components/auth/step-up-dialog";
import { RefreshCw, DownloadCloud, CheckCircle2, Loader2, AlertTriangle, RotateCcw } from "lucide-react";
import { cn } from "@/lib/utils";

const UPDATE_COMMAND = "cd gest-postgres && git pull && sudo ./setup.sh";
const ELEVATION_MS = 5 * 60 * 1000;

function formatDate(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleString("pt-BR");
}

export function UpdateChecker() {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [elevated, setElevated] = useState(false);
  const [stepUpOpen, setStepUpOpen] = useState(false);

  useEffect(() => {
    if (!elevated) return;
    const timer = setTimeout(() => setElevated(false), ELEVATION_MS);
    return () => clearTimeout(timer);
  }, [elevated]);

  const { data, isLoading, isFetching, refetch, isError, error } = useQuery({
    queryKey: ["update-check"],
    queryFn: () => api.checkUpdate(),
    enabled: open,
    staleTime: 60_000,
  });

  const statusQuery = useQuery({
    queryKey: ["update-apply-status"],
    queryFn: () => api.updateStatus(),
    enabled: open,
    retry: false,
    refetchInterval: (query) => (query.state.data?.status === "running" ? 3000 : false),
  });

  const applyMutation = useMutation({
    mutationFn: () => api.applyUpdate(),
    onSuccess: () => {
      toast.success("Atualização disparada — acompanhe o progresso abaixo");
      statusQuery.refetch();
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) {
        toast.message("Já tem uma atualização em andamento");
        statusQuery.refetch();
        return;
      }
      toast.error(e instanceof ApiError ? e.message : "Falha disparando atualização");
    },
  });

  function copyCommand() {
    navigator.clipboard.writeText(UPDATE_COMMAND).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    });
  }

  function handleApplyClick() {
    if (!elevated) {
      setStepUpOpen(true);
      return;
    }
    applyMutation.mutate();
  }

  const agentAvailable = !statusQuery.isError;
  const applyStatus = statusQuery.data?.status;
  const showApplySection = agentAvailable && data && !data.unknown && !data.up_to_date;

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="icon" title="Verificar atualização" />}>
        <DownloadCloud className="size-4" />
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5">
            <DownloadCloud className="size-4" />
            Atualização
          </DialogTitle>
        </DialogHeader>

        {isLoading ? (
          <p className="text-muted-foreground text-sm">Verificando...</p>
        ) : isError ? (
          <div className="flex flex-col gap-1">
            <p className="text-sm text-red-600">Não consegui checar atualização.</p>
            <p className="text-muted-foreground text-xs">
              {error instanceof ApiError
                ? error.message
                : "Falha alcançando o backend — confere se ele está rodando."}
            </p>
            <p className="text-muted-foreground text-xs">
              Motivo mais comum com conexão OK: a checagem usa a API pública do GitHub sem token
              (60 requisições/hora por IP) — se o IP do servidor já bateu esse limite, toda
              checagem falha por um tempo. Confirma direto no host:{" "}
              <code className="font-mono">curl -s https://api.github.com/rate_limit</code>
            </p>
          </div>
        ) : data?.unknown ? (
          <p className="text-muted-foreground text-sm">
            Esse binário foi buildado sem informação de commit (build manual, fora do{" "}
            <code className="font-mono">setup.sh</code>) — não dá pra comparar com o GitHub.
          </p>
        ) : (
          <div className="flex flex-col gap-3">
            {data?.up_to_date ? (
              <div className="flex items-center gap-2 text-sm text-emerald-600">
                <CheckCircle2 className="size-4" />
                Você já está na versão mais recente
              </div>
            ) : (
              <>
                <div className="flex items-center gap-2 text-sm">
                  <Badge variant="outline" className="border-amber-200 bg-amber-50 text-amber-700">
                    atualização disponível
                  </Badge>
                </div>
                {data?.latest_commit_message && (
                  <p className="text-sm">
                    <span className="font-medium">Último commit:</span> {data.latest_commit_message}
                    <br />
                    <span className="text-muted-foreground text-xs">
                      {data.latest_commit?.slice(0, 7)} · {formatDate(data.latest_commit_date)}
                    </span>
                  </p>
                )}
                <div>
                  <p className="text-muted-foreground mb-1 text-xs">
                    Ou rode isso no servidor pra atualizar na mão:
                  </p>
                  <div className="bg-muted flex items-center justify-between gap-2 rounded-md border p-2">
                    <code className="font-mono text-xs break-all">{UPDATE_COMMAND}</code>
                    <Button size="sm" variant="outline" onClick={copyCommand} className="shrink-0">
                      {copied ? "copiado" : "copiar"}
                    </Button>
                  </div>
                </div>
                {data?.compare_url && (
                  <a
                    href={data.compare_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-xs text-blue-600 hover:underline"
                  >
                    Ver o que mudou no GitHub
                  </a>
                )}
              </>
            )}

            {showApplySection && (
              <div className="border-t pt-3">
                {applyStatus === "running" ? (
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center gap-2 text-sm text-amber-700">
                      <Loader2 className="size-4 animate-spin" />
                      Atualizando... pode levar alguns minutos
                    </div>
                    <p className="text-muted-foreground text-xs">
                      A conexão pode cair por alguns segundos quando o backend reiniciar — é
                      esperado, não feche esta aba.
                    </p>
                    <LogTail text={statusQuery.data?.log_tail} />
                  </div>
                ) : applyStatus === "success" ? (
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center gap-2 text-sm text-emerald-600">
                      <CheckCircle2 className="size-4" />
                      Atualização concluída
                    </div>
                    <Button size="sm" onClick={() => window.location.reload()} className="self-start">
                      Recarregar página
                    </Button>
                  </div>
                ) : applyStatus === "failed" || applyStatus === "unknown" ? (
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center gap-2 text-sm text-red-600">
                      <AlertTriangle className="size-4" />
                      {applyStatus === "failed"
                        ? "Atualização falhou"
                        : "Atualização interrompida (host reiniciou ou o processo caiu no meio)"}
                    </div>
                    <LogTail text={statusQuery.data?.log_tail} />
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={applyMutation.isPending}
                      onClick={handleApplyClick}
                      className="self-start"
                    >
                      <RotateCcw className="size-3.5" />
                      Tentar de novo
                    </Button>
                  </div>
                ) : (
                  <Button size="sm" disabled={applyMutation.isPending} onClick={handleApplyClick}>
                    {applyMutation.isPending ? "Disparando..." : "Atualizar agora"}
                  </Button>
                )}
              </div>
            )}

            <p className="text-muted-foreground text-xs">
              Commit atual: <span className="font-mono">{data?.current_commit}</span>
            </p>
          </div>
        )}

        <Button
          size="sm"
          variant="outline"
          disabled={isFetching}
          onClick={() => refetch()}
          className="self-start"
        >
          <RefreshCw className={cn("size-3.5", isFetching && "animate-spin")} />
          Verificar de novo
        </Button>
      </DialogContent>

      {stepUpOpen && (
        <StepUpDialog
          onClose={() => setStepUpOpen(false)}
          onElevated={() => {
            setElevated(true);
            setStepUpOpen(false);
            applyMutation.mutate();
          }}
        />
      )}
    </Dialog>
  );
}

function LogTail({ text }: { text?: string }) {
  if (!text) return null;
  return (
    <pre className="bg-muted max-h-40 overflow-y-auto rounded-md border p-2 font-mono text-[11px] leading-relaxed whitespace-pre-wrap">
      {text}
    </pre>
  );
}
