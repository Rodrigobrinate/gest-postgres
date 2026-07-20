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
  const [mode, setMode] = useState<"proxy" | "redirect" | "external">("proxy");
  const [domain, setDomain] = useState("");
  const [targetContainer, setTargetContainer] = useState("");
  const [targetPort, setTargetPort] = useState(80);
  const [targetUrl, setTargetUrl] = useState("");
  const [tls, setTls] = useState(true);
  const [pathPrefix, setPathPrefix] = useState("/");
  const [stripPrefix, setStripPrefix] = useState(false);
  const [redirectTarget, setRedirectTarget] = useState("");
  const [redirectPermanent, setRedirectPermanent] = useState(true);
  const [httpsRedirect, setHttpsRedirect] = useState(false);
  const [certResolver, setCertResolver] = useState("");

  // Sem o gestpg-traefik habilitado, a única forma de rotear é via label no
  // container alvo, apontando pro Traefik externo já detectado (ex:
  // EasyPanel) — nunca criamos/recriamos esse Traefik.
  const viaLabels = !status?.enabled && !!status?.external_detected;
  const effectiveMode = viaLabels ? "proxy" : mode;

  const createRoute = useMutation({
    mutationFn: () => {
      if (effectiveMode === "proxy") {
        return api.createProxyRoute({
          domain,
          target_container: targetContainer,
          target_port: targetPort,
          tls,
          path_prefix: pathPrefix,
          strip_prefix: stripPrefix,
          https_redirect: httpsRedirect,
          via_labels: viaLabels,
          cert_resolver: viaLabels ? certResolver : undefined,
        });
      }
      if (effectiveMode === "external") {
        return api.createProxyRoute({
          domain,
          target_url: targetUrl,
          tls,
          path_prefix: pathPrefix,
          https_redirect: httpsRedirect,
        });
      }
      return api.createProxyRoute({
        domain,
        tls,
        path_prefix: pathPrefix,
        redirect_target: redirectTarget,
        redirect_permanent: redirectPermanent,
      });
    },
    onSuccess: () => {
      toast.success(`Rota "${domain}" criada`);
      setOpen(false);
      setDomain("");
      setTargetContainer("");
      setTargetUrl("");
      setRedirectTarget("");
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
              {!status.enabled && status.external_detected && (
                <Badge variant="outline" className="border-blue-200 bg-blue-50 text-blue-700">
                  externo detectado
                </Badge>
              )}
            </span>
            {status.enabled ? (
              <Button size="sm" variant="outline" disabled={disable.isPending} onClick={() => disable.mutate()}>
                {disable.isPending ? "Desabilitando..." : "Desabilitar"}
              </Button>
            ) : status.external_detected ? null : (
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
              : status.external_detected
                ? `Já existe um Traefik rodando (container "${status.external_container_name}") — provavelmente do EasyPanel ou outra stack. Nunca subimos um segundo: as rotas abaixo aplicam label Docker direto no container alvo, que esse Traefik já detecta sozinho.`
                : "Publica um container gerenciado num domínio próprio, com HTTPS automático via Let's Encrypt (desafio HTTP-01 — sem precisar de credencial de provedor de DNS)."}
          </p>
        </CardContent>
      </Card>

      {(status.enabled || status.external_detected) && (
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
                  {viaLabels ? (
                    <p className="text-muted-foreground text-xs">
                      Modo via labels só suporta proxy pra container (redirecionamento e destino
                      externo exigem o gestpg-traefik com file provider).
                    </p>
                  ) : (
                    <div className="flex items-center gap-4">
                      <label className="flex items-center gap-1.5 text-sm">
                        <input type="radio" checked={mode === "proxy"} onChange={() => setMode("proxy")} />
                        Proxy pra container
                      </label>
                      <label className="flex items-center gap-1.5 text-sm">
                        <input type="radio" checked={mode === "redirect"} onChange={() => setMode("redirect")} />
                        Redirecionamento
                      </label>
                      <label className="flex items-center gap-1.5 text-sm">
                        <input type="radio" checked={mode === "external"} onChange={() => setMode("external")} />
                        Destino externo
                      </label>
                    </div>
                  )}

                  <div className="grid grid-cols-2 gap-3">
                    <div className="grid gap-1.5">
                      <Label>Domínio</Label>
                      <Input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="app.seudominio.com" />
                    </div>
                    <div className="grid gap-1.5">
                      <Label>Caminho</Label>
                      <Input
                        value={pathPrefix}
                        onChange={(e) => setPathPrefix(e.target.value)}
                        placeholder="/"
                        className="font-mono text-xs"
                      />
                    </div>
                  </div>

                  {effectiveMode === "proxy" ? (
                    <>
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
                      {pathPrefix !== "/" && (
                        <label className="flex items-center gap-1.5 text-sm">
                          <input
                            type="checkbox"
                            checked={stripPrefix}
                            onChange={(e) => setStripPrefix(e.target.checked)}
                          />
                          Remover o prefixo do caminho antes de repassar pro container
                        </label>
                      )}
                    </>
                  ) : effectiveMode === "redirect" ? (
                    <>
                      <div className="grid gap-1.5">
                        <Label>URL de destino</Label>
                        <Input
                          value={redirectTarget}
                          onChange={(e) => setRedirectTarget(e.target.value)}
                          placeholder="https://outro-dominio.com"
                        />
                      </div>
                      <label className="flex items-center gap-1.5 text-sm">
                        <input
                          type="checkbox"
                          checked={redirectPermanent}
                          onChange={(e) => setRedirectPermanent(e.target.checked)}
                        />
                        Permanente (301) — desmarcado usa temporário (302)
                      </label>
                    </>
                  ) : (
                    <div className="grid gap-1.5">
                      <Label>URL do destino externo</Label>
                      <Input
                        value={targetUrl}
                        onChange={(e) => setTargetUrl(e.target.value)}
                        placeholder="http://203.0.113.10:9000"
                      />
                      <p className="text-muted-foreground text-xs">
                        Qualquer host:porta alcançável do servidor — não precisa ser um container
                        gerenciado por essa plataforma.
                      </p>
                    </div>
                  )}

                  <label className="flex items-center gap-1.5 text-sm">
                    <input type="checkbox" checked={tls} onChange={(e) => setTls(e.target.checked)} />
                    {viaLabels ? "HTTPS (via label tls no Traefik externo)" : "HTTPS automático (Let’s Encrypt)"}
                  </label>
                  {viaLabels && tls && (
                    <div className="grid gap-1.5">
                      <Label>Cert resolver do Traefik externo (opcional)</Label>
                      <Input
                        value={certResolver}
                        onChange={(e) => setCertResolver(e.target.value)}
                        placeholder="ex: letsencrypt"
                        className="font-mono text-xs"
                      />
                      <p className="text-muted-foreground text-xs">
                        Nome do certResolver já configurado NAQUELE Traefik (não temos como
                        descobrir sozinho — confira no compose/args dele). Vazio: label só pede TLS
                        sem resolver, funciona se o Traefik externo já tiver um default.
                      </p>
                    </div>
                  )}
                  {!viaLabels && mode !== "redirect" && tls && (
                    <label className="flex items-center gap-1.5 text-sm">
                      <input
                        type="checkbox"
                        checked={httpsRedirect}
                        onChange={(e) => setHttpsRedirect(e.target.checked)}
                      />
                      Redirecionar http:// pra https:// automaticamente
                    </label>
                  )}
                  {effectiveMode === "proxy" && (
                    <p className="text-muted-foreground text-xs">
                      {viaLabels
                        ? "Recria o container alvo com os labels do Traefik — nunca toca no Traefik externo. Se ele estiver numa rede diferente, conecta o alvo nela também (best-effort)."
                        : "Conecta o container alvo na rede gestpg-managed automaticamente se ele ainda não estiver nela."}
                    </p>
                  )}
                </div>
                <DialogFooter>
                  <Button
                    disabled={
                      createRoute.isPending ||
                      !domain.trim() ||
                      (effectiveMode === "proxy" && !targetContainer.trim()) ||
                      (effectiveMode === "redirect" && !redirectTarget.trim()) ||
                      (effectiveMode === "external" && !targetUrl.trim())
                    }
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
                      <span className="font-mono">
                        {r.domain}
                        {r.path_prefix !== "/" && r.path_prefix}
                      </span>
                      <Badge variant="outline">{r.tls ? "https" : "http"}</Badge>
                      {r.https_redirect && <Badge variant="outline">http→https</Badge>}
                      {r.via_labels && <Badge variant="outline">via label</Badge>}
                      <span className="text-muted-foreground font-mono text-xs">
                        {r.redirect_target
                          ? `↪ ${r.redirect_target} (${r.redirect_permanent ? "301" : "302"})`
                          : r.target_url
                            ? `→ ${r.target_url}`
                            : `→ ${r.target_container}:${r.target_port}${r.strip_prefix ? " (sem prefixo)" : ""}`}
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
