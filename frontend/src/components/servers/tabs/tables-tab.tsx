"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Table2, Trash2 } from "lucide-react";
import { CreateTableDialog } from "../create-table-dialog";
import { TableDataGrid } from "../table-data-grid";
import { TableTriggers } from "../table-triggers";
import { RetentionPolicies } from "../retention-policies";

export function TablesTab({ serverId, database }: { serverId: string; database: string }) {
  const { data: tables, isLoading } = useQuery({
    queryKey: ["servers", serverId, "tables", database],
    queryFn: () => api.listTables(serverId, database),
    enabled: !!database,
  });

  const [selected, setSelected] = useState<{ schema: string; name: string } | null>(null);

  function selectTable(schema: string, name: string) {
    setSelected({ schema, name });
  }

  const queryClient = useQueryClient();
  const dropTable = useMutation({
    mutationFn: (t: { schema: string; name: string }) => api.dropTable(serverId, database, t.schema, t.name),
    onSuccess: (_d, t) => {
      toast.success(`Tabela "${t.name}" excluída`);
      if (selected?.schema === t.schema && selected?.name === t.name) {
        setSelected(null);
      }
      queryClient.invalidateQueries({ queryKey: ["servers", serverId, "tables"] });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir tabela"),
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <CreateTableDialog serverId={serverId} database={database} />
      </div>
      <div className="grid grid-cols-[240px_1fr] gap-4">
      <Card className="h-fit">
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-4 text-sm">Carregando...</p>
          ) : !tables || tables.length === 0 ? (
            <p className="text-muted-foreground p-4 text-sm">
              Nenhuma tabela em &ldquo;{database}&rdquo;.
            </p>
          ) : (
            <ul className="divide-y">
              {tables.map((t) => (
                <li
                  key={`${t.schema}.${t.name}`}
                  className={cn(
                    "hover:bg-accent group flex items-center gap-1 pr-1",
                    selected?.schema === t.schema && selected?.name === t.name && "bg-accent"
                  )}
                >
                  <button
                    onClick={() => selectTable(t.schema, t.name)}
                    className="flex min-w-0 flex-1 items-center gap-2 px-3 py-2 text-left text-sm"
                  >
                    <Table2 className="text-muted-foreground size-3.5 shrink-0" />
                    <span className="truncate">
                      {t.schema}.{t.name}
                    </span>
                  </button>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="size-6 shrink-0 text-red-600 opacity-0 group-hover:opacity-100"
                    disabled={dropTable.isPending}
                    onClick={() => dropTable.mutate({ schema: t.schema, name: t.name })}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-0">
          {!selected ? (
            <p className="text-muted-foreground p-6 text-sm">Escolhe uma tabela pra ver os dados.</p>
          ) : (
            <>
              <TableDataGrid
                key={`${selected.schema}.${selected.name}`}
                serverId={serverId}
                database={database}
                schema={selected.schema}
                table={selected.name}
              />
              <TableTriggers
                serverId={serverId}
                database={database}
                schema={selected.schema}
                table={selected.name}
              />
            </>
          )}
        </CardContent>
      </Card>
      </div>

      <RetentionPolicies serverId={serverId} database={database} />
    </div>
  );
}
