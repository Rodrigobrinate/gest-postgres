"use client";

import { useMemo, useState } from "react";
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

function formatMB(v: number) {
  if (v >= 1024) return `${(v / 1024).toFixed(1)} GB`;
  return `${v.toFixed(0)} MB`;
}

function formatCount(v: number) {
  return String(Math.round(v));
}

// Paleta fixa (não a mesma cor dos outros gráficos, pra não confundir com
// CPU/memória/conexões/disco agregado) — repete em ciclo se tiver mais
// bancos que cores.
const PALETTE = ["#d97706", "#2563eb", "#059669", "#dc2626", "#7c3aed", "#0891b2", "#db2777", "#65a30d"];

type ByDatabase = Record<string, number> | undefined;

// databaseNames extrai o conjunto de bancos que apareceram em QUALQUER ponto
// do histórico — um banco criado/excluído no meio da janela ainda aparece
// (com linha começando/parando onde tem dado).
function databaseNames(points: MetricPoint[], field: (p: MetricPoint) => ByDatabase): string[] {
  const set = new Set<string>();
  for (const p of points) {
    for (const name of Object.keys(field(p) ?? {})) set.add(name);
  }
  return Array.from(set).sort();
}

function toChartData(points: MetricPoint[], field: (p: MetricPoint) => ByDatabase) {
  return points.map((p) => ({ timestamp: p.timestamp, ...field(p) }));
}

function Chart({
  data,
  names,
  height,
  field,
  formatValue,
}: {
  data: MetricPoint[];
  names: string[];
  height: number;
  field: (p: MetricPoint) => ByDatabase;
  formatValue: (v: number) => string;
}) {
  const chartData = useMemo(() => toChartData(data, field), [data, field]);
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={chartData} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
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
          width={48}
          tickFormatter={(v) => formatValue(Number(v))}
        />
        <Tooltip
          labelFormatter={(v) => formatTime(String(v))}
          formatter={(value, name) => [formatValue(Number(value)), name]}
          cursor={{ stroke: "var(--muted-foreground)", strokeDasharray: "3 3", strokeWidth: 1 }}
          contentStyle={{
            fontSize: 12,
            borderRadius: 8,
            border: "1px solid var(--border)",
            background: "var(--popover)",
          }}
        />
        <Legend wrapperStyle={{ fontSize: 12 }} />
        {names.map((name, i) => (
          <Line
            key={name}
            type="monotone"
            dataKey={name}
            name={name}
            stroke={PALETTE[i % PALETTE.length]}
            strokeWidth={2}
            dot={false}
            connectNulls
            activeDot={{ r: 5, stroke: "var(--background)", strokeWidth: 2 }}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}

// PerDatabaseChart é a base reaproveitada por qualquer métrica que a
// plataforma consiga abrir por banco (hoje: disco e conexões) — uma linha
// por banco, mesmo card clicável-pra-ampliar com seletor de período.
function PerDatabaseChart({
  history,
  title,
  field,
  formatValue,
  note,
  serverId,
}: {
  history: MetricPoint[];
  title: string;
  field: (p: MetricPoint) => ByDatabase;
  formatValue: (v: number) => string;
  note: string;
  serverId?: string;
}) {
  const names = useMemo(() => databaseNames(history, field), [history, field]);
  const hasData = history.length >= 2 && names.length > 0;
  const [open, setOpen] = useState(false);
  const [range, setRange] = useState<RangeKey>("1h");
  const extended = isBackendRange(range);

  const { data: extendedHistory, isFetching: extendedLoading } = useQuery({
    queryKey: ["servers", serverId, "metrics-history", range],
    queryFn: () => api.metricsHistory(serverId!, range),
    enabled: open && extended && !!serverId,
  });

  const zoomedData = extended
    ? (extendedHistory ?? [])
    : filterByRange(history, rangeMs(range), (p) => new Date(p.timestamp).getTime());
  const zoomedNames = extended ? databaseNames(zoomedData, field) : names;
  const gapNote = extended
    ? coverageGapNote(zoomedData[0]?.timestamp, zoomedData[zoomedData.length - 1]?.timestamp, range)
    : null;

  return (
    <>
      <Card
        className={hasData ? "cursor-pointer transition-colors hover:bg-muted/40" : undefined}
        onClick={() => hasData && setOpen(true)}
        title={hasData ? "Clique pra ampliar e mudar o período" : undefined}
      >
        <CardHeader>
          <CardTitle className="text-sm font-medium">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          {!hasData ? (
            <div className="text-muted-foreground flex h-[220px] items-center justify-center text-xs">
              Coletando dados... (amostra a cada 15s)
            </div>
          ) : (
            <Chart data={history} names={names} height={220} field={field} formatValue={formatValue} />
          )}
        </CardContent>
      </Card>

      {open && (
        <Dialog open onOpenChange={setOpen}>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>{title}</DialogTitle>
            </DialogHeader>
            <TimeRangeButtons value={range} onChange={setRange} />
            {extended && extendedLoading ? (
              <div className="text-muted-foreground flex h-[340px] items-center justify-center text-xs">
                Carregando histórico...
              </div>
            ) : extended && zoomedNames.length === 0 ? (
              <div className="text-muted-foreground flex h-[340px] items-center justify-center text-xs">
                Sem detalhe por banco além das últimas 24h — o resumo por hora guarda só o
                agregado, não a quebra por banco.
              </div>
            ) : (
              <Chart data={zoomedData} names={zoomedNames} height={340} field={field} formatValue={formatValue} />
            )}
            <p className="text-muted-foreground text-xs">{note}</p>
            {gapNote && <p className="text-amber-600 text-xs">{gapNote}</p>}
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}

// DatabaseSizeChart mostra o tamanho em disco de CADA banco ao longo do
// tempo, uma linha por banco — Postgres não rastreia RAM por banco
// individual (shared_buffers é um pool só, compartilhado pelo cluster
// inteiro), então tamanho em disco (pg_database_size) é o proxy real mais
// próximo de "uso de recurso por banco" que dá pra medir.
export function DatabaseSizeChart({ history, serverId }: { history: MetricPoint[]; serverId?: string }) {
  return (
    <PerDatabaseChart
      history={history}
      serverId={serverId}
      title="Disco por banco"
      field={(p) => p.database_sizes_mb}
      formatValue={formatMB}
      note="Postgres não mede RAM por banco individual (shared_buffers é um pool só do cluster) — esse gráfico usa tamanho em disco (pg_database_size), o proxy real mais próximo por banco. Histórico em memória (~1h a 15s/amostra), reseta se o backend reiniciar."
    />
  );
}

// ConnectionsPerDatabaseChart abre o total de "Conexões" (gráfico agregado
// ao lado) por banco — pg_stat_activity tem coluna datname, então essa
// quebra é dado real (não um proxy como o disco acima).
export function ConnectionsPerDatabaseChart({
  history,
  serverId,
}: {
  history: MetricPoint[];
  serverId?: string;
}) {
  return (
    <PerDatabaseChart
      history={history}
      serverId={serverId}
      title="Conexões por banco"
      field={(p) => p.connections_by_database}
      formatValue={formatCount}
      note="Conta sessões ativas em pg_stat_activity agrupadas por datname. Histórico em memória (~1h a 15s/amostra), reseta se o backend reiniciar."
    />
  );
}
