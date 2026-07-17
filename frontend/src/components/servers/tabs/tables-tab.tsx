"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { formatCell, cn } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Table2 } from "lucide-react";

const PAGE_SIZE = 50;

export function TablesTab({ serverId, database }: { serverId: string; database: string }) {
  const { data: tables, isLoading } = useQuery({
    queryKey: ["servers", serverId, "tables", database],
    queryFn: () => api.listTables(serverId, database),
    enabled: !!database,
  });

  const [selected, setSelected] = useState<{ schema: string; name: string } | null>(null);
  const [page, setPage] = useState(0);

  const { data: rowsResult, isLoading: rowsLoading } = useQuery({
    queryKey: ["servers", serverId, "tableRows", database, selected?.schema, selected?.name, page],
    queryFn: () =>
      api.tableRows(serverId, selected!.schema, selected!.name, {
        database,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
    enabled: !!selected,
  });

  function selectTable(schema: string, name: string) {
    setSelected({ schema, name });
    setPage(0);
  }

  return (
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
                <li key={`${t.schema}.${t.name}`}>
                  <button
                    onClick={() => selectTable(t.schema, t.name)}
                    className={cn(
                      "hover:bg-accent flex w-full items-center gap-2 px-3 py-2 text-left text-sm",
                      selected?.schema === t.schema &&
                        selected?.name === t.name &&
                        "bg-accent"
                    )}
                  >
                    <Table2 className="text-muted-foreground size-3.5 shrink-0" />
                    <span className="truncate">
                      {t.schema}.{t.name}
                    </span>
                  </button>
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
          ) : rowsLoading || !rowsResult ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : (
            <>
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      {rowsResult.columns.map((c) => (
                        <TableHead key={c}>{c}</TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rowsResult.rows.map((row, i) => (
                      <TableRow key={i}>
                        {row.map((cell, j) => (
                          <TableCell key={j} className="font-mono text-xs whitespace-nowrap">
                            {formatCell(cell)}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <div className="flex items-center justify-between border-t p-3">
                <p className="text-muted-foreground text-xs">
                  {rowsResult.total_rows} linha(s) — página {page + 1}
                </p>
                <div className="flex gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={page === 0}
                    onClick={() => setPage((p) => p - 1)}
                  >
                    Anterior
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={(page + 1) * PAGE_SIZE >= rowsResult.total_rows}
                    onClick={() => setPage((p) => p + 1)}
                  >
                    Próxima
                  </Button>
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
