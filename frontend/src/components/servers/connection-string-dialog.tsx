"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type ManagedServer } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { KeyRound, Copy, Eye, EyeOff, RefreshCw } from "lucide-react";

export function ConnectionStringDialog({ server }: { server: ManagedServer }) {
  const [open, setOpen] = useState(false);
  const [reveal, setReveal] = useState(false);
  const [selectedUser, setSelectedUser] = useState(server.username);
  const [selectedDb, setSelectedDb] = useState(server.database_name);
  // Senha de qualquer role que não seja o superuser nunca fica guardada na
  // plataforma (mesma regra da aba Usuários) — só existe aqui, em memória,
  // depois de gerar uma nova nessa mesma sessão de dialog aberto.
  const [otherPassword, setOtherPassword] = useState<string | null>(null);

  const isSuperuser = selectedUser === server.username;

  const { data: roles } = useQuery({
    queryKey: ["servers", server.id, "roles"],
    queryFn: () => api.listRoles(server.id),
    enabled: open,
  });
  const { data: databases } = useQuery({
    queryKey: ["servers", server.id, "databases"],
    queryFn: () => api.listDatabases(server.id),
    enabled: open,
  });

  const { data: superuserPw, isLoading: superuserPwLoading } = useQuery({
    queryKey: ["servers", server.id, "password"],
    queryFn: () => api.getPassword(server.id),
    enabled: open && isSuperuser,
    staleTime: Infinity,
  });

  // Trocar de usuário invalida qualquer senha revelada antes — mostrar a
  // senha do usuário anterior rotulada como se fosse do novo seria um bug
  // sério (credencial errada indo pra área de transferência de alguém). Feito
  // direto no handler de troca (não via useEffect) — é o único lugar que
  // muda selectedUser, então não precisa de sincronização reativa pra isso.
  function changeUser(user: string) {
    setSelectedUser(user);
    setOtherPassword(null);
    setReveal(false);
  }

  const queryClient = useQueryClient();
  const rotateSuperuser = useMutation({
    mutationFn: () => api.rotateSuperuserPassword(server.id),
    onSuccess: (result) => {
      queryClient.setQueryData(["servers", server.id, "password"], result);
      setReveal(true);
      toast.success("Senha regenerada — connection string atualizada abaixo");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao regenerar senha"),
  });

  const rotateOther = useMutation({
    mutationFn: () => api.rotateRolePassword(server.id, selectedUser),
    onSuccess: (result) => {
      setOtherPassword(result.password);
      setReveal(true);
      toast.success("Senha gerada — copie agora, não fica salva");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao gerar senha"),
  });

  const isLoading = isSuperuser ? superuserPwLoading : false;
  const password = isSuperuser ? (superuserPw?.password ?? "") : (otherPassword ?? "");
  const hasPassword = !!password;

  const host = typeof window !== "undefined" ? window.location.hostname : "localhost";
  const connectionString = `postgresql://${selectedUser}:${password}@${host}:${server.host_port}/${selectedDb}`;
  const masked = `postgresql://${selectedUser}:${hasPassword ? "•".repeat(10) : "(gere uma senha)"}@${host}:${server.host_port}/${selectedDb}`;

  function copy() {
    if (!hasPassword) return;
    navigator.clipboard.writeText(connectionString);
    toast.success("Connection string copiada");
  }

  const rotating = isSuperuser ? rotateSuperuser.isPending : rotateOther.isPending;
  function rotate() {
    if (isSuperuser) rotateSuperuser.mutate();
    else rotateOther.mutate();
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" />}>
        <KeyRound className="size-4" />
        Connection string
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Connection string</DialogTitle>
          <DialogDescription>
            Pra conectar de fora da plataforma (psql, DBeaver, sua aplicação, etc).
          </DialogDescription>
        </DialogHeader>

        <div className="grid grid-cols-2 gap-3 text-sm">
          <Field label="Host" value={host} />
          <Field label="Porta" value={String(server.host_port)} />
          <div>
            <p className="text-muted-foreground text-xs">Usuário</p>
            <Select value={selectedUser} onValueChange={(v) => v && changeUser(v)}>
              <SelectTrigger className="h-7 w-full text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={server.username}>{server.username} (superuser)</SelectItem>
                {(roles ?? [])
                  .filter((r) => r.name !== server.username)
                  .map((r) => (
                    <SelectItem key={r.name} value={r.name}>
                      {r.name}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>
          <div>
            <p className="text-muted-foreground text-xs">Banco</p>
            <Select value={selectedDb} onValueChange={(v) => v && setSelectedDb(v)}>
              <SelectTrigger className="h-7 w-full text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(databases ?? [server.database_name]).map((db) => (
                  <SelectItem key={db} value={db}>
                    {db}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="grid gap-2">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground text-xs">String completa</span>
            <div className="flex gap-1">
              <Button
                size="sm"
                variant="ghost"
                disabled={rotating}
                onClick={rotate}
                title={
                  isSuperuser
                    ? "Gerar uma nova senha pro superuser (invalida a atual)"
                    : "Gerar uma nova senha pra esse usuário (invalida a atual, não fica salva na plataforma)"
                }
              >
                <RefreshCw className="size-3.5" />
                {rotating ? "Gerando..." : isSuperuser ? "Regenerar senha" : "Gerar senha"}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={!hasPassword}
                onClick={() => setReveal((r) => !r)}
                title={reveal ? "Ocultar senha" : "Revelar senha"}
              >
                {reveal ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
                {reveal ? "Ocultar" : "Revelar senha"}
              </Button>
            </div>
          </div>
          <div className="bg-muted flex items-center gap-2 rounded-md border p-2">
            <code className="flex-1 overflow-x-auto font-mono text-xs whitespace-nowrap">
              {isLoading ? "Carregando..." : reveal && hasPassword ? connectionString : masked}
            </code>
            <Button size="icon" variant="ghost" onClick={copy} disabled={isLoading || !hasPassword} title="Copiar">
              <Copy className="size-4" />
            </Button>
          </div>
          {!isSuperuser && !hasPassword && (
            <p className="text-muted-foreground text-xs">
              Senha de usuário que não é o superuser não fica guardada na plataforma — clique em
              &ldquo;Gerar senha&rdquo; pra criar uma nova e revelar.
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="font-mono text-sm">{value}</p>
    </div>
  );
}
