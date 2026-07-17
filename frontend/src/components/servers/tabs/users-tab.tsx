"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type Privilege } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { CreateRoleDialog } from "../create-role-dialog";
import { ChevronDown, ChevronRight, ShieldCheck, Trash2 } from "lucide-react";

export function UsersTab({ serverId, database }: { serverId: string; database: string }) {
  const { data: roles, isLoading } = useQuery({
    queryKey: ["servers", serverId, "roles"],
    queryFn: () => api.listRoles(serverId),
  });

  const [expanded, setExpanded] = useState<string | null>(null);

  const queryClient = useQueryClient();
  const remove = useMutation({
    mutationFn: (name: string) => api.dropRole(serverId, name),
    onSuccess: (_d, name) => {
      toast.success(`${name} excluído`);
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "roles"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir usuário"),
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
                  {expanded === r.name && (
                    <RolePrivileges serverId={serverId} database={database} role={r.name} />
                  )}
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
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
  const { data: privs, isLoading } = useQuery({
    queryKey: ["servers", serverId, "role-privileges", database, role],
    queryFn: () => api.rolePrivileges(serverId, role, database),
    enabled: !!database,
  });

  const queryClient = useQueryClient();
  const toggle = useMutation({
    mutationFn: (args: { schema: string; table: string; privilege: Privilege; grant: boolean }) =>
      api.setPrivilege(serverId, role, database, args.schema, args.table, args.privilege, args.grant),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "role-privileges"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar permissão"),
  });

  return (
    <div className="bg-muted/30 border-t px-4 py-3">
      <div className="mb-2 flex items-center gap-1.5 text-xs font-medium">
        <ShieldCheck className="size-3.5" />
        Permissões em &ldquo;{database}&rdquo;
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
