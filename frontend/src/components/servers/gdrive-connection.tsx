"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, API_URL } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { HardDriveDownload } from "lucide-react";

export function GDriveConnection() {
  const { data: status, isLoading } = useQuery({
    queryKey: ["gdrive-status"],
    queryFn: () => api.gdriveStatus(),
    refetchInterval: 5_000,
  });
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["gdrive-status"] });

  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");

  const saveConfig = useMutation({
    mutationFn: () => api.setGDriveConfig(clientId, clientSecret),
    onSuccess: () => {
      toast.success("Configuração salva");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao salvar"),
  });

  const connect = useMutation({
    mutationFn: () => api.gdriveAuthUrl(),
    onSuccess: (result) => {
      window.open(result.url, "_blank", "noopener,noreferrer,width=520,height=650");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao gerar link de autorização"),
  });

  const disconnect = useMutation({
    mutationFn: () => api.gdriveDisconnect(),
    onSuccess: () => {
      toast.success("Google Drive desconectado");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao desconectar"),
  });

  if (isLoading || !status) {
    return null;
  }

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center justify-between">
          <span className="flex items-center gap-1.5 text-sm font-medium">
            <HardDriveDownload className="size-4" />
            Google Drive
            {status.connected && <Badge variant="outline">conectado</Badge>}
          </span>
          {status.connected && (
            <Button size="sm" variant="outline" disabled={disconnect.isPending} onClick={() => disconnect.mutate()}>
              Desconectar
            </Button>
          )}
        </div>

        {status.connected ? (
          <p className="text-muted-foreground mt-2 text-xs">
            Conta: <span className="font-mono">{status.account_email}</span> — backups com storage
            &ldquo;Google Drive&rdquo; sobem pra pasta &ldquo;gest-postgres-backups&rdquo; nessa conta.
          </p>
        ) : (
          <div className="mt-3 flex flex-col gap-3">
            <p className="text-muted-foreground text-xs">
              Precisa de um app OAuth próprio no{" "}
              <a
                href="https://console.cloud.google.com/apis/credentials"
                target="_blank"
                rel="noopener noreferrer"
                className="underline"
              >
                Google Cloud Console
              </a>{" "}
              (tipo &ldquo;Web application&rdquo;) com a Drive API habilitada e a URI de redirecionamento
              autorizada apontando pra{" "}
              <span className="font-mono">
                {API_URL}/api/v1/gdrive/callback
              </span>
              .
            </p>
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label>Client ID</Label>
                <Input value={clientId} onChange={(e) => setClientId(e.target.value)} className="h-8 text-xs" />
              </div>
              <div className="grid gap-1.5">
                <Label>Client Secret</Label>
                <Input
                  type="password"
                  value={clientSecret}
                  onChange={(e) => setClientSecret(e.target.value)}
                  className="h-8 text-xs"
                />
              </div>
            </div>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                disabled={saveConfig.isPending || !clientId.trim() || !clientSecret.trim()}
                onClick={() => saveConfig.mutate()}
              >
                {saveConfig.isPending ? "Salvando..." : "Salvar credenciais"}
              </Button>
              {status.configured && (
                <Button size="sm" disabled={connect.isPending} onClick={() => connect.mutate()}>
                  {connect.isPending ? "Gerando link..." : "Conectar ao Google"}
                </Button>
              )}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
