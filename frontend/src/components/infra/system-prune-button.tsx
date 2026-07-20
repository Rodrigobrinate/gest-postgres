"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Sparkles } from "lucide-react";

// "docker system prune -f" — container parado, rede sem uso, imagem
// dangling e cache de build. De propósito SEM volume (nunca -v/-a) — ver
// internal/infra/system_prune.go.
export function SystemPruneButton() {
  const [log, setLog] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const prune = useMutation({
    mutationFn: () => api.systemPrune(),
    onSuccess: (result) => {
      toast.success("Limpeza concluída");
      setLog(result.log || "Nada pra limpar.");
      queryClient.invalidateQueries({ queryKey: ["infra-containers"] });
      queryClient.invalidateQueries({ queryKey: ["infra-networks"] });
      queryClient.invalidateQueries({ queryKey: ["infra-volumes"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha na limpeza"),
  });

  return (
    <>
      <Button
        size="sm"
        variant="outline"
        disabled={prune.isPending}
        onClick={() => {
          if (confirm("Remove containers parados, redes sem uso, imagens sem tag e cache de build. Volumes nunca são tocados. Continuar?")) {
            prune.mutate();
          }
        }}
      >
        <Sparkles className="size-4" />
        {prune.isPending ? "Limpando..." : "Limpar sistema"}
      </Button>

      {log !== null && (
        <Dialog open onOpenChange={(v) => !v && setLog(null)}>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle>Limpeza concluída</DialogTitle>
            </DialogHeader>
            <pre className="bg-muted max-h-96 overflow-auto rounded-md p-3 text-xs">{log}</pre>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
