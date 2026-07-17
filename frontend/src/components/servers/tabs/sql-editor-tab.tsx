"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type QueryResult } from "@/lib/api";
import { formatCell } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Play } from "lucide-react";

const DEFAULT_SQL = "SELECT * FROM pg_catalog.pg_tables LIMIT 10;";

export function SqlEditorTab({ serverId, database }: { serverId: string; database: string }) {
  const [sql, setSql] = useState(DEFAULT_SQL);
  const [result, setResult] = useState<QueryResult | null>(null);

  const run = useMutation({
    mutationFn: () => api.runQuery(serverId, database, sql),
    onSuccess: (data) => {
      setResult(data);
      toast.success(`${data.row_count} linha(s) em ${data.duration_ms}ms`);
    },
    onError: (e) => {
      setResult(null);
      toast.error(e instanceof ApiError ? e.message : "Falha ao executar query");
    },
  });

  function handleRun() {
    if (!sql.trim()) return;
    run.mutate();
  }

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardContent className="flex flex-col gap-3 p-4">
          <Textarea
            value={sql}
            onChange={(e) => setSql(e.target.value)}
            rows={8}
            className="font-mono text-sm"
            placeholder="SELECT * FROM ..."
            onKeyDown={(e) => {
              if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                e.preventDefault();
                handleRun();
              }
            }}
          />
          <div className="flex items-center justify-between">
            <p className="text-muted-foreground text-xs">
              Ctrl/Cmd+Enter roda a query · banco: {database}
            </p>
            <Button onClick={handleRun} disabled={run.isPending}>
              <Play className="size-4" />
              {run.isPending ? "Rodando..." : "Rodar"}
            </Button>
          </div>
        </CardContent>
      </Card>

      {result && (
        <Card>
          <CardContent className="p-0">
            {result.columns.length === 0 ? (
              <p className="text-muted-foreground p-6 text-sm">
                {result.command_tag || "Comando executado"} — {result.duration_ms}ms
              </p>
            ) : (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      {result.columns.map((c) => (
                        <TableHead key={c}>{c}</TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {result.rows.map((row, i) => (
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
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
