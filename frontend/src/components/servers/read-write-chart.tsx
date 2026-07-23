"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  Legend,
  XAxis,
  YAxis,
  CartesianGrid,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { api, type MetricPoint } from "@/lib/api";
import { TimeRangeButtons, filterByRange, isBackendRange, rangeMs, coverageGapNote, type RangeKey } from "./timerange-buttons";

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}

function formatTuples(v: number) {
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k/s`;
  return `${v.toFixed(1)}/s`;
}

type Row = { timestamp: string; read: number; write: number };

function toRows(points: MetricPoint[]): Row[] {
  return points.map((p) => ({
    timestamp: p.timestamp,
    read: p.read_tuples_per_sec,
    write: p.write_tuples_per_sec,
  }));
}

function Chart({ data, height }: { data: Row[]; height: number }) {
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={data} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border)" />
        <XAxis
          dataKey="timestamp"
          tickFormatter={formatTime}
          tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
          axisLine={{ stroke: "var(--border)" }}
          tickLine={false}
          minTickGap={40}
        />
        <YAxis
          tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
          axisLine={false}
          tickLine={false}
          width={44}
          tickFormatter={(v) => formatTuples(Number(v))}
        />
        <Tooltip
          labelFormatter={(v) => formatTime(String(v))}
          formatter={(value, name) => [formatTuples(Number(value)), name === "read" ? "Leitura" : "Escrita"]}
          cursor={{ stroke: "var(--muted-foreground)", strokeDasharray: "3 3", strokeWidth: 1 }}
          contentStyle={{
            fontSize: 12,
            borderRadius: 8,
            border: "1px solid var(--border)",
            background: "var(--popover)",
          }}
        />
        <Legend formatter={(v) => (v === "read" ? "Leitura" : "Escrita")} wrapperStyle={{ fontSize: 12 }} />
        <Line type="monotone" dataKey="read" stroke="#2563eb" strokeWidth={2} dot={false} isAnimationActive={false} activeDot={{ r: 4 }} />
        <Line type="monotone" dataKey="write" stroke="#d97706" strokeWidth={2} dot={false} isAnimationActive={false} activeDot={{ r: 4 }} />
      </LineChart>
    </ResponsiveContainer>
  );
}

// ReadWriteChart mostra atividade de leitura/escrita de linha (tuple) do
// servidor — pg_stat_database.tup_returned+tup_fetched (leitura) e
// tup_inserted+tup_updated+tup_deleted (escrita), somado em todo banco
// não-template, convertido pra taxa (por segundo) por delta entre polls —
// nunca acumulado, pedido explícito do usuário depois do mesmo ajuste em
// IOPS.
export function ReadWriteChart({ history, serverId }: { history: MetricPoint[]; serverId?: string }) {
  const rows = toRows(history);
  const hasData = rows.length >= 2;
  const [open, setOpen] = useState(false);
  const [range, setRange] = useState<RangeKey>("1h");
  const extended = isBackendRange(range);

  const { data: extendedHistory, isFetching: extendedLoading } = useQuery({
    queryKey: ["servers", serverId, "metrics-history", range],
    queryFn: () => api.metricsHistory(serverId!, range),
    enabled: open && extended && !!serverId,
  });

  const zoomedRows = extended
    ? toRows(extendedHistory ?? [])
    : filterByRange(rows, rangeMs(range), (p) => new Date(p.timestamp).getTime());
  const gapNote = extended
    ? coverageGapNote(zoomedRows[0]?.timestamp, zoomedRows[zoomedRows.length - 1]?.timestamp, range)
    : null;

  return (
    <>
      <Card
        className={hasData ? "cursor-pointer transition-colors hover:bg-muted/40" : undefined}
        onClick={() => hasData && setOpen(true)}
        title={hasData ? "Clique pra ampliar e mudar o período" : undefined}
      >
        <CardHeader>
          <CardTitle className="text-sm font-medium">Leituras e escritas</CardTitle>
        </CardHeader>
        <CardContent>
          {!hasData ? (
            <div className="text-muted-foreground flex h-[220px] items-center justify-center text-xs">
              Coletando dados... (amostra a cada 15s)
            </div>
          ) : (
            <Chart data={rows} height={220} />
          )}
        </CardContent>
      </Card>

      {open && (
        <Dialog open onOpenChange={setOpen}>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>Leituras e escritas</DialogTitle>
            </DialogHeader>
            <TimeRangeButtons value={range} onChange={setRange} />
            {extended && extendedLoading ? (
              <div className="text-muted-foreground flex h-[340px] items-center justify-center text-xs">
                Carregando histórico...
              </div>
            ) : (
              <Chart data={zoomedRows} height={340} />
            )}
            <p className="text-muted-foreground text-xs">
              {extended
                ? "Dado agregado por hora além das últimas 24h (média/máximo) — persistido, sobrevive a reinício do backend."
                : "Histórico em memória (~1h a 15s/amostra), reseta se o backend reiniciar."}{" "}
              Linhas lidas (scan sequencial ou por índice) e linhas escritas (INSERT/UPDATE/DELETE)
              por segundo, somado em todo banco não-template — via <code>pg_stat_database</code>.
            </p>
            {gapNote && <p className="text-amber-600 text-xs">{gapNote}</p>}
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
