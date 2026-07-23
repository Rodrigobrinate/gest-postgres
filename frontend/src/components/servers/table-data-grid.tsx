"use client";

import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type RowFilter, type RowFilterOp } from "@/lib/api";
import { formatCell, cn } from "@/lib/utils";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ArrowUp, ArrowDown, ArrowUpDown, Filter, X, Trash2 } from "lucide-react";
import { AddRowDialog } from "./add-row-dialog";

const PAGE_SIZE = 50;

const FILTER_OPS: { value: RowFilterOp; label: string }[] = [
  { value: "eq", label: "=" },
  { value: "neq", label: "≠" },
  { value: "gt", label: ">" },
  { value: "gte", label: "≥" },
  { value: "lt", label: "<" },
  { value: "lte", label: "≤" },
  { value: "contains", label: "contém" },
  { value: "is_null", label: "é nulo" },
  { value: "is_not_null", label: "não é nulo" },
];

function opLabel(op: RowFilterOp) {
  return FILTER_OPS.find((o) => o.value === op)?.label ?? op;
}

// TableDataGrid — grid de dados tipo Prisma Studio (pedido explícito do
// usuário, 2026-07-22): ordenar clicando no cabeçalho, filtro simples por
// coluna, editar célula com duplo clique, adicionar/excluir linha. Montado
// com `key={schema.table}` pelo componente pai (TablesTab) — troca de
// tabela reseta todo o estado local (página/ordenação/filtro/edição em
// andamento) de graça, sem precisar de useEffect pra sincronizar.
export function TableDataGrid({
  serverId,
  database,
  schema,
  table,
}: {
  serverId: string;
  database: string;
  schema: string;
  table: string;
}) {
  const [page, setPage] = useState(0);
  const [sortColumn, setSortColumn] = useState<string | null>(null);
  const [sortDesc, setSortDesc] = useState(false);
  const [filters, setFilters] = useState<RowFilter[]>([]);
  const [filterOpen, setFilterOpen] = useState(false);
  const [draftColumn, setDraftColumn] = useState("");
  const [draftOp, setDraftOp] = useState<RowFilterOp>("eq");
  const [draftValue, setDraftValue] = useState("");

  const [editing, setEditing] = useState<{ row: number; column: string } | null>(null);
  const [editValue, setEditValue] = useState("");
  const [editNull, setEditNull] = useState(false);
  // Duplo clique numa célula, deu Enter/Escape, DEPOIS o blur do input
  // dispara de novo (remover o input do DOM no meio de foco costuma gerar
  // um blur nativo com o handler "antigo" do React, closure velha e tudo) —
  // esse ref (não state, lido sempre fresco independente de closure) evita
  // o commit/replay duplicado.
  const skipBlurRef = useRef(false);

  const queryClient = useQueryClient();

  const { data: columns } = useQuery({
    queryKey: ["servers", serverId, "tableColumns", database, schema, table],
    queryFn: () => api.tableColumns(serverId, database, schema, table),
  });

  const { data: rowsResult, isLoading } = useQuery({
    queryKey: ["servers", serverId, "tableRows", database, schema, table, page, sortColumn, sortDesc, filters],
    queryFn: () =>
      api.tableRows(serverId, schema, table, {
        database,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        sort: sortColumn ?? undefined,
        dir: sortDesc ? "desc" : "asc",
        filters,
      }),
  });

  function invalidateRows() {
    queryClient.invalidateQueries({ queryKey: ["servers", serverId, "tableRows", database, schema, table] });
  }

  const updateCell = useMutation({
    mutationFn: (input: { column: string; value: unknown; pk: Record<string, unknown> }) =>
      api.updateTableRow(serverId, database, schema, table, input.column, input.value, input.pk),
    onSuccess: () => invalidateRows(),
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao salvar célula"),
  });

  const deleteRow = useMutation({
    mutationFn: (pk: Record<string, unknown>) => api.deleteTableRow(serverId, database, schema, table, pk),
    onSuccess: () => {
      toast.success("Linha excluída");
      invalidateRows();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir linha"),
  });

  const pkColumns = (columns ?? []).filter((c) => c.is_primary_key).map((c) => c.name);
  const canEdit = pkColumns.length > 0;
  const columnNames = columns?.map((c) => c.name) ?? rowsResult?.columns ?? [];

  function rowPK(row: unknown[]): Record<string, unknown> | null {
    if (!rowsResult || pkColumns.length === 0) return null;
    const pk: Record<string, unknown> = {};
    for (const col of pkColumns) {
      const idx = rowsResult.columns.indexOf(col);
      if (idx === -1) return null;
      pk[col] = row[idx];
    }
    return pk;
  }

  function toggleSort(col: string) {
    setPage(0);
    if (sortColumn !== col) {
      setSortColumn(col);
      setSortDesc(false);
    } else if (!sortDesc) {
      setSortDesc(true);
    } else {
      setSortColumn(null);
      setSortDesc(false);
    }
  }

  function addFilter() {
    if (!draftColumn) return;
    setFilters((f) => [...f, { column: draftColumn, op: draftOp, value: draftValue }]);
    setDraftValue("");
    setPage(0);
    setFilterOpen(false);
  }

  function removeFilter(i: number) {
    setFilters((f) => f.filter((_, idx) => idx !== i));
    setPage(0);
  }

  function startEdit(rowIndex: number, column: string, value: unknown) {
    if (!canEdit) return;
    skipBlurRef.current = false;
    setEditing({ row: rowIndex, column });
    setEditNull(value === null);
    setEditValue(value === null || value === undefined ? "" : formatCell(value));
  }

  function cancelEdit() {
    skipBlurRef.current = true;
    setEditing(null);
  }

  function commitEdit(row: unknown[]) {
    if (!editing) return;
    skipBlurRef.current = true;
    const pk = rowPK(row);
    const column = editing.column;
    const value = editNull ? null : editValue;
    setEditing(null);
    if (!pk) return;
    updateCell.mutate({ column, value, pk });
  }

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-2 border-b p-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <Button size="sm" variant="outline" onClick={() => setFilterOpen((o) => !o)}>
            <Filter className="size-3.5" />
            Filtro
          </Button>
          {filters.map((f, i) => (
            <span key={i} className="bg-muted flex items-center gap-1 rounded-full px-2.5 py-1 font-mono text-xs">
              {f.column} {opLabel(f.op)} {f.op !== "is_null" && f.op !== "is_not_null" ? f.value : ""}
              <button onClick={() => removeFilter(i)} className="text-muted-foreground hover:text-foreground">
                <X className="size-3" />
              </button>
            </span>
          ))}
        </div>
        {columns && (
          <AddRowDialog serverId={serverId} database={database} schema={schema} table={table} columns={columns} />
        )}
      </div>

      {filterOpen && (
        <div className="bg-muted/30 flex flex-wrap items-center gap-2 border-b p-2">
          <Select value={draftColumn} onValueChange={(v) => v && setDraftColumn(v)}>
            <SelectTrigger className="w-40">
              <SelectValue placeholder="coluna" />
            </SelectTrigger>
            <SelectContent>
              {columnNames.map((c) => (
                <SelectItem key={c} value={c}>
                  {c}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={draftOp} onValueChange={(v) => v && setDraftOp(v as RowFilterOp)}>
            <SelectTrigger className="w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {FILTER_OPS.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {draftOp !== "is_null" && draftOp !== "is_not_null" && (
            <Input
              className="w-40"
              placeholder="valor"
              value={draftValue}
              onChange={(e) => setDraftValue(e.target.value)}
            />
          )}
          <Button size="sm" onClick={addFilter} disabled={!draftColumn}>
            Adicionar
          </Button>
        </div>
      )}

      {isLoading || !rowsResult ? (
        <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
      ) : (
        <>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  {rowsResult.columns.map((c) => (
                    <TableHead key={c}>
                      <button onClick={() => toggleSort(c)} className="hover:text-foreground flex items-center gap-1">
                        {c}
                        {sortColumn === c ? (
                          sortDesc ? (
                            <ArrowDown className="size-3" />
                          ) : (
                            <ArrowUp className="size-3" />
                          )
                        ) : (
                          <ArrowUpDown className="text-muted-foreground/50 size-3" />
                        )}
                      </button>
                    </TableHead>
                  ))}
                  {canEdit && <TableHead className="w-8" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {rowsResult.rows.map((row, i) => {
                  const pk = rowPK(row);
                  return (
                    <TableRow key={i} className="group">
                      {row.map((cell, j) => {
                        const col = rowsResult.columns[j];
                        const isEditing = editing?.row === i && editing.column === col;
                        return (
                          <TableCell
                            key={j}
                            className={cn("font-mono text-xs whitespace-nowrap", canEdit && "cursor-text")}
                            onDoubleClick={() => startEdit(i, col, cell)}
                          >
                            {isEditing ? (
                              <div className="flex items-center gap-1.5">
                                <input
                                  autoFocus
                                  className="border-input bg-background w-32 rounded border px-1 py-0.5 text-xs"
                                  value={editValue}
                                  disabled={editNull}
                                  onChange={(e) => setEditValue(e.target.value)}
                                  onKeyDown={(e) => {
                                    if (e.key === "Enter") commitEdit(row);
                                    if (e.key === "Escape") cancelEdit();
                                  }}
                                  onBlur={() => {
                                    if (skipBlurRef.current) {
                                      skipBlurRef.current = false;
                                      return;
                                    }
                                    commitEdit(row);
                                  }}
                                />
                                <label
                                  className="text-muted-foreground flex items-center gap-0.5 text-[10px]"
                                  onMouseDown={(e) => e.preventDefault()}
                                >
                                  <input
                                    type="checkbox"
                                    checked={editNull}
                                    onChange={(e) => setEditNull(e.target.checked)}
                                  />
                                  NULL
                                </label>
                              </div>
                            ) : (
                              <span className={cell === null ? "text-muted-foreground italic" : undefined}>
                                {formatCell(cell)}
                              </span>
                            )}
                          </TableCell>
                        );
                      })}
                      {canEdit && (
                        <TableCell className="w-8 p-0">
                          <Button
                            size="icon"
                            variant="ghost"
                            className="size-6 text-red-600 opacity-0 group-hover:opacity-100"
                            disabled={deleteRow.isPending || !pk}
                            onClick={() => pk && deleteRow.mutate(pk)}
                          >
                            <Trash2 className="size-3.5" />
                          </Button>
                        </TableCell>
                      )}
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
          <div className="flex items-center justify-between border-t p-3">
            <p className="text-muted-foreground text-xs">
              {rowsResult.total_rows} linha(s) — página {page + 1}
              {!canEdit && " · tabela sem chave primária, edição/exclusão de linha desativada"}
            </p>
            <div className="flex gap-2">
              <Button size="sm" variant="outline" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
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
    </>
  );
}
