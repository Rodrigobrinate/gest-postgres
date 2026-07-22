"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type Privilege } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { CreateRoleDialog } from "../create-role-dialog";
import { ChevronDown, ChevronRight, KeyRound, ShieldCheck, Trash2, Eye, Pencil, Ban } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export function UsersTab({ serverId, database }: { serverId: string; database: string }) {
  const { data: roles, isLoading } = useQuery({
    queryKey: ["servers", serverId, "roles"],
    queryFn: () => api.listRoles(serverId),
  });

  const [expanded, setExpanded] = useState<string | null>(null);
  const [newPassword, setNewPassword] = useState<{ role: string; password: string } | null>(null);

  const queryClient = useQueryClient();
  const remove = useMutation({
    mutationFn: (name: string) => api.dropRole(serverId, name),
    onSuccess: (_d, name) => {
      toast.success(`${name} excluído`);
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "roles"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir usuário"),
  });

  const rotate = useMutation({
    mutationFn: (name: string) => api.rotateRolePassword(serverId, name),
    onSuccess: (result, name) => setNewPassword({ role: name, password: result.password }),
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao regenerar senha"),
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <CreateRoleDialog serverId={serverId} />
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !roles || roles.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhum usuário.</p>
          ) : (
            <ul className="divide-y">
              {roles.map((r) => (
                <li key={r.name}>
                  <div className="flex items-center justify-between gap-3 p-3">
                    <button
                      className="flex min-w-0 flex-1 items-center gap-2 text-left"
                      onClick={() => setExpanded((cur) => (cur === r.name ? null : r.name))}
                    >
                      {expanded === r.name ? (
                        <ChevronDown className="text-muted-foreground size-4 shrink-0" />
                      ) : (
                        <ChevronRight className="text-muted-foreground size-4 shrink-0" />
                      )}
                      <span className="font-mono text-sm font-medium">{r.name}</span>
                      <div className="flex gap-1">
                        {r.superuser && <Badge variant="outline">superuser</Badge>}
                        {r.create_db && <Badge variant="outline">createdb</Badge>}
                        {r.create_role && <Badge variant="outline">createrole</Badge>}
                        {!r.can_login && <Badge variant="outline">sem login</Badge>}
                      </div>
                    </button>
                    <div className="flex shrink-0 gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Regenerar senha"
                        disabled={rotate.isPending}
                        onClick={() => rotate.mutate(r.name)}
                      >
                        <KeyRound className="size-4" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Excluir"
                        className="text-red-600 hover:text-red-700"
                        onClick={() => remove.mutate(r.name)}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </div>
                  </div>
                  {expanded === r.name && (
                    <RolePrivileges serverId={serverId} database={database} role={r.name} />
                  )}
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <Dialog open={!!newPassword} onOpenChange={(v) => !v && setNewPassword(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Nova senha de &ldquo;{newPassword?.role}&rdquo;</DialogTitle>
          </DialogHeader>
          <p className="text-muted-foreground text-xs">
            Essa senha não fica guardada na plataforma — copie agora, você não vai poder ver de novo.
          </p>
          <code className="bg-muted block rounded-md border p-2 font-mono text-sm break-all">
            {newPassword?.password}
          </code>
          <DialogFooter>
            <Button
              onClick={() => {
                if (newPassword) {
                  navigator.clipboard.writeText(newPassword.password);
                  toast.success("Copiado");
                }
              }}
            >
              Copiar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

const PRIVILEGES: Privilege[] = ["select", "insert", "update", "delete"];

function RolePrivileges({
  serverId,
  database,
  role,
}: {
  serverId: string;
  database: string;
  role: string;
}) {
  // Estado próprio, não o `database` (ativo na página inteira) do pai — é
  // isso que deixa dar uma olhada/mexer no acesso da role em QUALQUER banco
  // do servidor sem precisar trocar o banco ativo da página toda.
  const [selectedDb, setSelectedDb] = useState(database);

  const { data: databases } = useQuery({
    queryKey: ["servers", serverId, "databases"],
    queryFn: () => api.listDatabases(serverId),
  });

  const { data: privs, isLoading } = useQuery({
    queryKey: ["servers", serverId, "role-privileges", selectedDb, role],
    queryFn: () => api.rolePrivileges(serverId, role, selectedDb),
    enabled: !!selectedDb,
  });

  const queryClient = useQueryClient();
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "role-privileges"] });

  const toggle = useMutation({
    mutationFn: (args: { schema: string; table: string; privilege: Privilege; grant: boolean }) =>
      api.setPrivilege(serverId, role, selectedDb, args.schema, args.table, args.privilege, args.grant),
    onSuccess: invalidate,
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar permissão"),
  });

  const setLevel = useMutation({
    mutationFn: (level: "none" | "read" | "write") => api.setAccessLevel(serverId, role, selectedDb, level),
    onSuccess: (_d, level) => {
      const label = level === "none" ? "Acesso removido" : level === "read" ? "Somente leitura aplicada" : "Leitura e escrita aplicada";
      toast.success(`${label} em "${selectedDb}"`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao aplicar preset de acesso"),
  });

  return (
    <div className="bg-muted/30 border-t px-4 py-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-1.5 text-xs font-medium">
          <ShieldCheck className="size-3.5" />
          Permissões em
          <Select value={selectedDb} onValueChange={(v) => v && setSelectedDb(v)}>
            <SelectTrigger className="h-6 w-40 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(databases ?? [database]).map((db) => (
                <SelectItem key={db} value={db}>
                  {db}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex gap-1">
          <Button
            size="sm"
            variant="outline"
            className="h-6 text-xs"
            disabled={setLevel.isPending}
            onClick={() => setLevel.mutate("read")}
            title="Concede SELECT em todas as tabelas do banco de uma vez"
          >
            <Eye className="size-3" />
            Somente leitura
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="h-6 text-xs"
            disabled={setLevel.isPending}
            onClick={() => setLevel.mutate("write")}
            title="Concede SELECT/INSERT/UPDATE/DELETE em todas as tabelas do banco de uma vez"
          >
            <Pencil className="size-3" />
            Leitura e escrita
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="h-6 text-xs text-red-600 hover:text-red-700"
            disabled={setLevel.isPending}
            onClick={() => setLevel.mutate("none")}
            title="Revoga tudo nesse banco"
          >
            <Ban className="size-3" />
            Nenhum acesso
          </Button>
        </div>
      </div>
      {isLoading ? (
        <p className="text-muted-foreground text-xs">Carregando...</p>
      ) : !privs || privs.length === 0 ? (
        <p className="text-muted-foreground text-xs">Nenhuma tabela nesse banco.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-muted-foreground">
                <th className="p-1 text-left font-normal">Tabela</th>
                {PRIVILEGES.map((p) => (
                  <th key={p} className="p-1 text-center font-normal uppercase">
                    {p}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {privs.map((t) => (
                <tr key={`${t.schema}.${t.table}`} className="border-t">
                  <td className="p-1 font-mono">
                    {t.schema}.{t.table}
                  </td>
                  {PRIVILEGES.map((p) => (
                    <td key={p} className="p-1 text-center">
                      <input
                        type="checkbox"
                        checked={t[p]}
                        disabled={toggle.isPending}
                        onChange={(e) =>
                          toggle.mutate({
                            schema: t.schema,
                            table: t.table,
                            privilege: p,
                            grant: e.target.checked,
                          })
                        }
                      />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
