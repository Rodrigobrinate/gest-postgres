"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type UserRole } from "@/lib/api";
import { useCurrentUser, useIsAdmin } from "./current-user";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Plus, Trash2, Users, KeyRound } from "lucide-react";

export function UsersManager() {
  const isAdmin = useIsAdmin();
  if (!isAdmin) return null;
  return <UsersManagerDialog />;
}

function UsersManagerDialog() {
  const [open, setOpen] = useState(false);
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
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="icon" title="Usuários" />}>
        <Users className="size-4" />
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5">
            <Users className="size-4" />
            Usuários
          </DialogTitle>
        </DialogHeader>

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
          className="grid grid-cols-[1fr_1fr_auto_auto] items-end gap-2 border-t pt-3"
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
        <p className="text-muted-foreground text-xs">
          Viewer só enxerga — qualquer criação/edição/exclusão, terminal e escrita no file manager
          exigem admin.
        </p>
      </DialogContent>
    </Dialog>
  );
}
