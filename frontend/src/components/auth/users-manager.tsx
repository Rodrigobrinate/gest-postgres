"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type UserRole, type SessionInfo, type LoginAttempt } from "@/lib/api";
import { useCurrentUser, useIsAdmin } from "./current-user";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Plus, Trash2, Users, KeyRound, ShieldCheck } from "lucide-react";
import { cn } from "@/lib/utils";

export function UsersManager() {
  const isAdmin = useIsAdmin();
  if (!isAdmin) return null;
  return <UsersManagerDialog />;
}

function UsersManagerDialog() {
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="icon" title="Usuários e sessões" />}>
        <Users className="size-4" />
      </DialogTrigger>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5">
            <ShieldCheck className="size-4" />
            Usuários e sessões
          </DialogTitle>
        </DialogHeader>

        <Tabs defaultValue="users">
          <TabsList>
            <TabsTrigger value="users">Usuários</TabsTrigger>
            <TabsTrigger value="sessions">Sessões</TabsTrigger>
            <TabsTrigger value="attempts">Tentativas de login</TabsTrigger>
          </TabsList>
          <TabsContent value="users" className="pt-3">
            <UsersTabContent open={open} />
          </TabsContent>
          <TabsContent value="sessions" className="pt-3">
            <SessionsTabContent open={open} />
          </TabsContent>
          <TabsContent value="attempts" className="pt-3">
            <LoginAttemptsTabContent open={open} />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

function UsersTabContent({ open }: { open: boolean }) {
  const currentUser = useCurrentUser();

  const { data: users, isLoading } = useQuery({
    queryKey: ["users"],
    queryFn: () => api.listUsers(),
    enabled: open,
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["users"] });

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<UserRole>("viewer");

  const create = useMutation({
    mutationFn: () => api.createUser(username, password, role),
    onSuccess: () => {
      toast.success("Usuário criado");
      setUsername("");
      setPassword("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar usuário"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteUser(id),
    onSuccess: () => {
      toast.success("Usuário removido");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover usuário"),
  });

  const [resettingId, setResettingId] = useState<string | null>(null);
  const [newPassword, setNewPassword] = useState("");
  const resetPassword = useMutation({
    mutationFn: (id: string) => api.resetUserPassword(id, newPassword),
    onSuccess: () => {
      toast.success("Senha trocada");
      setResettingId(null);
      setNewPassword("");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao trocar senha"),
  });

  return (
    <>
      {isLoading ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
      ) : (
        <ul className="divide-y rounded-md border">
          {(users ?? []).map((u) => (
            <li key={u.id} className="flex flex-col gap-1.5 px-3 py-2 text-sm">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{u.username}</span>
                  <Badge variant={u.role === "admin" ? "default" : "secondary"}>{u.role}</Badge>
                  {u.username === currentUser?.username && (
                    <span className="text-muted-foreground text-xs">(você)</span>
                  )}
                </div>
                <div className="flex items-center gap-1">
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    title="Trocar senha"
                    onClick={() => setResettingId(resettingId === u.id ? null : u.id)}
                  >
                    <KeyRound className="size-3.5" />
                  </Button>
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    className="text-red-600"
                    title="Excluir"
                    disabled={remove.isPending}
                    onClick={() => {
                      if (confirm(`Excluir usuário "${u.username}"?`)) remove.mutate(u.id);
                    }}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              </div>
              {resettingId === u.id && (
                <div className="flex items-center gap-1.5">
                  <Input
                    type="password"
                    placeholder="Senha nova"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    className="h-7 text-xs"
                  />
                  <Button
                    size="xs"
                    disabled={resetPassword.isPending || !newPassword}
                    onClick={() => resetPassword.mutate(u.id)}
                  >
                    Salvar
                  </Button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}

      <form
        className="mt-3 grid grid-cols-[1fr_1fr_auto_auto] items-end gap-2 border-t pt-3"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <div className="grid gap-1.5">
          <Label>Usuário</Label>
          <Input value={username} onChange={(e) => setUsername(e.target.value)} />
        </div>
        <div className="grid gap-1.5">
          <Label>Senha</Label>
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
        </div>
        <div className="grid gap-1.5">
          <Label>Papel</Label>
          <Select value={role} onValueChange={(v) => v && setRole(v as UserRole)}>
            <SelectTrigger className="w-28">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="viewer">Viewer</SelectItem>
              <SelectItem value="admin">Admin</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button type="submit" disabled={create.isPending || !username.trim() || !password.trim()}>
          <Plus className="size-4" />
        </Button>
      </form>
      <p className="text-muted-foreground mt-2 text-xs">
        Viewer só enxerga — qualquer criação/edição/exclusão, terminal e escrita no file manager
        exigem admin.
      </p>
    </>
  );
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString("pt-BR");
}

// truncateUA — user-agent cru é comprido demais pra caber numa linha da
// lista; corta e deixa o valor completo no title (tooltip nativo) pra quem
// precisar do detalhe (ex: distinguir dois logins do mesmo IP por
// navegador/dispositivo).
function truncateUA(ua: string) {
  if (!ua) return "—";
  return ua.length > 60 ? ua.slice(0, 60) + "…" : ua;
}

function SessionsTabContent({ open }: { open: boolean }) {
  const [view, setView] = useState<"active" | "history">("active");
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["auth-sessions", view],
    queryFn: () => (view === "active" ? api.listSessions() : api.sessionHistory()),
    enabled: open,
    // Sessão ativa muda com frequência (last_seen_at anda a cada
    // requisição de quem tá com a aba aberta) — poll curto só nessa visão;
    // histórico é só consulta pontual, sem sentido reatualizar sozinho.
    refetchInterval: view === "active" ? 10_000 : false,
  });

  const revoke = useMutation({
    mutationFn: (id: string) => api.revokeSession(id),
    onSuccess: () => {
      toast.success("Sessão encerrada");
      queryClient.invalidateQueries({ queryKey: ["auth-sessions"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao encerrar sessão"),
  });

  // "expirada" depende da hora atual, que não pode ser lida direto no corpo
  // do render (Date.now() ali é impuro) — medida num efeito e guardada em
  // state; 0 na primeira pintura só faz nenhuma sessão aparecer "expirada"
  // por um instante, corrigido assim que o efeito roda.
  const [now, setNow] = useState(0);
  useEffect(() => {
    // Deferido (não direto no corpo do efeito) — mesmo padrão já usado na
    // aba de Logs pro mesmo tipo de aviso do linter.
    const raf = requestAnimationFrame(() => setNow(Date.now()));
    return () => cancelAnimationFrame(raf);
  }, [data]);

  function status(s: SessionInfo): { label: string; variant: "default" | "secondary" | "destructive" } {
    if (s.revoked_at) return { label: "encerrada", variant: "secondary" };
    if (now > 0 && new Date(s.expires_at).getTime() < now) return { label: "expirada", variant: "secondary" };
    if (s.online) return { label: "online", variant: "default" };
    return { label: "ativa", variant: "secondary" };
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex gap-1.5">
        <button
          onClick={() => setView("active")}
          className={cn(
            "rounded-full border px-2.5 py-0.5 text-xs",
            view === "active" ? "border-foreground bg-foreground text-background" : "border-border text-muted-foreground"
          )}
        >
          Ativas
        </button>
        <button
          onClick={() => setView("history")}
          className={cn(
            "rounded-full border px-2.5 py-0.5 text-xs",
            view === "history" ? "border-foreground bg-foreground text-background" : "border-border text-muted-foreground"
          )}
        >
          Histórico
        </button>
      </div>

      {isLoading ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
      ) : !data || data.length === 0 ? (
        <p className="text-muted-foreground text-sm">
          {view === "active" ? "Nenhuma sessão ativa." : "Sem histórico ainda."}
        </p>
      ) : (
        <ul className="max-h-96 divide-y overflow-y-auto rounded-md border">
          {data.map((s) => {
            const st = status(s);
            return (
              <li key={s.id} className="flex items-center justify-between gap-3 px-3 py-2 text-sm">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{s.username}</span>
                    <Badge variant={s.role === "admin" ? "default" : "secondary"}>{s.role}</Badge>
                    <Badge variant={st.variant}>{st.label}</Badge>
                    {s.current && <span className="text-muted-foreground text-xs">(esta sessão)</span>}
                  </div>
                  <p className="text-muted-foreground truncate font-mono text-xs" title={s.user_agent}>
                    {s.ip_address || "—"} · {truncateUA(s.user_agent)}
                  </p>
                  <p className="text-muted-foreground text-xs">
                    entrou {formatDate(s.created_at)} · última atividade {formatDate(s.last_seen_at)}
                    {s.revoked_at && ` · encerrada ${formatDate(s.revoked_at)}`}
                  </p>
                </div>
                {!s.revoked_at && (now === 0 || new Date(s.expires_at).getTime() > now) && (
                  <Button
                    size="xs"
                    variant="outline"
                    className="shrink-0 text-red-600"
                    disabled={revoke.isPending}
                    onClick={() => revoke.mutate(s.id)}
                  >
                    Encerrar
                  </Button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

function LoginAttemptsTabContent({ open }: { open: boolean }) {
  const [filter, setFilter] = useState<"all" | "success" | "failure">("all");

  const { data, isLoading } = useQuery({
    queryKey: ["login-attempts"],
    queryFn: () => api.listLoginAttempts(),
    enabled: open,
  });

  const filtered = (data ?? []).filter((a: LoginAttempt) => {
    if (filter === "success") return a.success;
    if (filter === "failure") return !a.success;
    return true;
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex gap-1.5">
        {(["all", "success", "failure"] as const).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={cn(
              "rounded-full border px-2.5 py-0.5 text-xs",
              filter === f ? "border-foreground bg-foreground text-background" : "border-border text-muted-foreground"
            )}
          >
            {f === "all" ? "Todas" : f === "success" ? "Sucesso" : "Falha"}
          </button>
        ))}
      </div>

      {isLoading ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
      ) : filtered.length === 0 ? (
        <p className="text-muted-foreground text-sm">Nenhuma tentativa registrada ainda.</p>
      ) : (
        <ul className="max-h-96 divide-y overflow-y-auto rounded-md border">
          {filtered.map((a) => (
            <li key={a.id} className="flex items-center justify-between gap-3 px-3 py-2 text-sm">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{a.username}</span>
                  <Badge variant={a.success ? "default" : "destructive"}>
                    {a.success ? "sucesso" : "falha"}
                  </Badge>
                </div>
                <p className="text-muted-foreground truncate font-mono text-xs" title={a.user_agent}>
                  {a.ip_address} · {truncateUA(a.user_agent)}
                </p>
              </div>
              <span className="text-muted-foreground shrink-0 text-xs">{formatDate(a.created_at)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
