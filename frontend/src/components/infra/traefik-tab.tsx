"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
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
  DialogTrigger,
} from "@/components/ui/dialog";
import { Globe, Plus, Trash2 } from "lucide-react";

export function TraefikTab() {
  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ["traefik-status"],
    queryFn: () => api.traefikStatus(),
    refetchInterval: 10_000,
  });

  const { data: routes, isLoading: routesLoading } = useQuery({
    queryKey: ["proxy-routes"],
    queryFn: () => api.listProxyRoutes(),
    enabled: !!status?.enabled,
  });

  const queryClient = useQueryClient();
  const invalidateStatus = () => queryClient.invalidateQueries({ queryKey: ["traefik-status"] });
  const invalidateRoutes = () => queryClient.invalidateQueries({ queryKey: ["proxy-routes"] });

  const [email, setEmail] = useState("");
  const enable = useMutation({
    mutationFn: () => api.enableTraefik(email),
    onSuccess: () => {
      toast.success("Traefik habilitado");
      invalidateStatus();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao habilitar Traefik"),
  });

  const disable = useMutation({
    mutationFn: () => api.disableTraefik(),
    onSuccess: () => {
      toast.success("Traefik desabilitado");
      invalidateStatus();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao desabilitar Traefik"),
  });

  const [open, setOpen] = useState(false);
  const [domain, setDomain] = useState("");
  const [targetContainer, setTargetContainer] = useState("");
  const [targetPort, setTargetPort] = useState(80);
  const [tls, setTls] = useState(true);

  const createRoute = useMutation({
    mutationFn: () => api.createProxyRoute({ domain, target_container: targetContainer, target_port: targetPort, tls }),
    onSuccess: () => {
      toast.success(`Rota "${domain}" criada`);
      setOpen(false);
      setDomain("");
      setTargetContainer("");
      invalidateRoutes();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar rota"),
  });

  const removeRoute = useMutation({
    mutationFn: (id: string) => api.removeProxyRoute(id),
    onSuccess: () => {
      toast.success("Rota removida");
      invalidateRoutes();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover rota"),
  });

  if (statusLoading || !status) {
    return <p className="text-muted-foreground p-6 text-sm">Carregando...</p>;
  }

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardContent className="p-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="flex items-center gap-1.5 text-sm font-medium">
              <Globe className="size-4" />
              Traefik (proxy reverso + Let&apos;s Encrypt)
              {status.enabled && (
                <Badge variant="outline" className={status.running ? "border-emerald-200 bg-emerald-50 text-emerald-700" : ""}>
                  {status.running ? "rodando" : "parado"}
                </Badge>
              )}
            </span>
            {status.enabled ? (
              <Button size="sm" variant="outline" disabled={disable.isPending} onClick={() => disable.mutate()}>
                {disable.isPending ? "Desabilitando..." : "Desabilitar"}
              </Button>
            ) : (
              <div className="flex items-center gap-2">
                <Input
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="admin@seudominio.com"
                  className="h-8 w-56 text-xs"
                />
                <Button size="sm" disabled={enable.isPending || !email.trim()} onClick={() => enable.mutate()}>
                  {enable.isPending ? "Habilitando..." : "Habilitar"}
                </Button>
              </div>
            )}
          </div>
          <p className="text-muted-foreground mt-2 text-xs">
            {status.enabled
              ? `Publica 80/443 no host. Certificados automáticos via HTTP-01 (e-mail: ${status.acme_email}) — precisa do domínio já apontando pro IP desse servidor antes de criar a rota.`
              : "Publica um container gerenciado num domínio próprio, com HTTPS automático via Let's Encrypt (desafio HTTP-01 — sem precisar de credencial de provedor de DNS)."}
          </p>
        </CardContent>
      </Card>

      {status.enabled && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-base">Rotas de domínio</CardTitle>
            <Dialog open={open} onOpenChange={setOpen}>
              <DialogTrigger render={<Button size="sm" />}>
                <Plus className="size-4" />
                Nova rota
              </DialogTrigger>
              <DialogContent className="sm:max-w-md">
                <DialogHeader>
                  <DialogTitle>Nova rota de domínio</DialogTitle>
                </DialogHeader>
                <div className="grid gap-3 py-2">
                  <div className="grid gap-1.5">
                    <Label>Domínio</Label>
                    <Input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="app.seudominio.com" />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="grid gap-1.5">
                      <Label>Container alvo</Label>
                      <Input
                        value={targetContainer}
                        onChange={(e) => setTargetContainer(e.target.value)}
                        placeholder="meu-stack-web-1"
                      />
                    </div>
                    <div className="grid gap-1.5">
                      <Label>Porta</Label>
                      <Input
                        type="number"
                        value={targetPort}
                        onChange={(e) => setTargetPort(Number(e.target.value))}
                      />
                    </div>
                  </div>
                  <label className="flex items-center gap-1.5 text-sm">
                    <input type="checkbox" checked={tls} onChange={(e) => setTls(e.target.checked)} />
                    HTTPS automático (Let&apos;s Encrypt)
                  </label>
                  <p className="text-muted-foreground text-xs">
                    Conecta o container alvo na rede gestpg-managed automaticamente se ele ainda não
                    estiver nela.
                  </p>
                </div>
                <DialogFooter>
                  <Button
                    disabled={createRoute.isPending || !domain.trim() || !targetContainer.trim()}
                    onClick={() => createRoute.mutate()}
                  >
                    {createRoute.isPending ? "Criando..." : "Criar"}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </CardHeader>
          <CardContent className="p-0">
            {routesLoading ? (
              <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
            ) : !routes || routes.length === 0 ? (
              <p className="text-muted-foreground p-6 text-sm">Nenhuma rota configurada.</p>
            ) : (
              <ul className="divide-y">
                {routes.map((r) => (
                  <li key={r.id} className="flex items-center justify-between px-4 py-2 text-sm">
                    <div className="flex items-center gap-2">
                      <span className="font-mono">{r.domain}</span>
                      <Badge variant="outline">{r.tls ? "https" : "http"}</Badge>
                      <span className="text-muted-foreground font-mono text-xs">
                        → {r.target_container}:{r.target_port}
                      </span>
                    </div>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="text-red-600"
                      disabled={removeRoute.isPending}
                      onClick={() => removeRoute.mutate(r.id)}
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
