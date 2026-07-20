"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ShieldAlert } from "lucide-react";

// Reconfirmação de senha antes de destravar uma ação de risco (hoje só o
// gerenciador de arquivos do host) — mesmo padrão de confirm-then-mutate da
// RestoreDialog de backup-tab.tsx, mas chamando POST /api/v1/auth/step-up
// em vez de disparar a ação diretamente.
export function StepUpDialog({ onClose, onElevated }: { onClose: () => void; onElevated: () => void }) {
  const [password, setPassword] = useState("");

  const stepUp = useMutation({
    mutationFn: () => api.stepUp(password),
    onSuccess: () => {
      toast.success("Zona de risco destravada por 5 minutos");
      onElevated();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao confirmar senha"),
  });

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5">
            <ShieldAlert className="size-4" />
            Confirme sua senha
          </DialogTitle>
        </DialogHeader>
        <form
          className="grid gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            stepUp.mutate();
          }}
        >
          <div className="grid gap-1.5">
            <Label htmlFor="stepup-password">Senha</Label>
            <Input
              id="stepup-password"
              type="password"
              autoFocus
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button type="submit" disabled={stepUp.isPending || !password}>
              {stepUp.isPending ? "Confirmando..." : "Confirmar"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
