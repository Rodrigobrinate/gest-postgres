"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import CodeMirror from "@uiw/react-codemirror";
import { sql as sqlLang } from "@codemirror/lang-sql";
import { keymap } from "@codemirror/view";
import { Prec } from "@codemirror/state";
import { api, ApiError, type QueryResult } from "@/lib/api";
import { formatCell } from "@/lib/utils";
import { useQueryHistory } from "@/lib/use-query-history";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Play, History } from "lucide-react";
import { cn } from "@/lib/utils";

const DEFAULT_SQL = "SELECT * FROM pg_catalog.pg_tables LIMIT 10;";

export function SqlEditorTab({ serverId, database }: { serverId: string; database: string }) {
  const [sqlText, setSqlText] = useState(DEFAULT_SQL);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [showHistory, setShowHistory] = useState(false);
  const { history, push } = useQueryHistory(`sql-history:${serverId}`);

  const run = useMutation({
    mutationFn: () => api.runQuery(serverId, database, sqlText),
    onSuccess: (data) => {
      setResult(data);
      push(sqlText);
      toast.success(`${data.row_count} linha(s) em ${data.duration_ms}ms`);
    },
    onError: (e) => {
      setResult(null);
      toast.error(e instanceof ApiError ? e.message : "Falha ao executar query");
    },
  });

  function handleRun() {
    if (!sqlText.trim()) return;
    run.mutate();
  }

  // Prec.highest garante que isso vence o keymap padrão do basicSetup — sem
  // isso, Ctrl/Cmd+Enter só inseria uma linha nova em vez de rodar a query.
  const runKeymap = Prec.highest(
    keymap.of([
      {
        key: "Mod-Enter",
        run: () => {
          handleRun();
          return true;
        },
      },
    ])
  );

  return (
    <div className="flex gap-4">
      <div className="flex flex-1 flex-col gap-4">
        <Card>
          <CardContent className="flex flex-col gap-3 p-4">
            <div className="overflow-hidden rounded-md border">
              <CodeMirror
                value={sqlText}
                height="200px"
                extensions={[sqlLang(), runKeymap]}
                onChange={(value) => setSqlText(value)}
                basicSetup={{ lineNumbers: true, foldGutter: false }}
                className="text-sm"
              />
            </div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <p className="text-muted-foreground text-xs">
                  Ctrl/Cmd+Enter roda a query · banco: {database}
                </p>
                {history.length > 0 && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setShowHistory((v) => !v)}
                  >
                    <History className="size-3.5" />
                    Histórico ({history.length})
                  </Button>
                )}
              </div>
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

      {showHistory && (
        <Card className="h-fit w-80 shrink-0">
          <CardContent className="p-0">
            <div className="border-b p-3 text-sm font-medium">Histórico de queries</div>
            <ul className="max-h-[400px] divide-y overflow-y-auto">
              {history.map((q, i) => (
                <li key={i}>
                  <button
                    className={cn(
                      "hover:bg-accent w-full truncate px-3 py-2 text-left font-mono text-xs"
                    )}
                    title={q}
                    onClick={() => setSqlText(q)}
                  >
                    {q}
                  </button>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
