"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  CartesianGrid,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { api } from "@/lib/api";
import { TimeRangeButtons, filterByRange, isBackendRange, rangeMs, coverageGapNote, type RangeKey } from "./timerange-buttons";

function formatTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}

type PointWithTimestamp = { timestamp: string };

type Props<T extends PointWithTimestamp> = {
  title: string;
  data: T[];
  dataKey: keyof T;
  color: string;
  unit?: string;
  formatValue?: (v: number) => string;
  // Informar isso liga os períodos 24h/7d/30d — sem serverId, o modal só
  // recorta o buffer em memória que já tinha (comportamento de sempre).
  serverId?: string;
};

function Chart<T extends PointWithTimestamp>({
  data,
  dataKey,
  color,
  unit,
  title,
  formatValue,
  height,
}: Props<T> & { height: number }) {
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
          width={40}
        />
        <Tooltip
          labelFormatter={(v) => formatTime(String(v))}
          formatter={(value) => {
            const n = Number(value);
            return [formatValue ? formatValue(n) : `${n.toFixed(1)}${unit ?? ""}`, title];
          }}
          cursor={{ stroke: "var(--muted-foreground)", strokeDasharray: "3 3", strokeWidth: 1 }}
          contentStyle={{
            fontSize: 12,
            borderRadius: 8,
            border: "1px solid var(--border)",
            background: "var(--popover)",
          }}
        />
        <Line
          type="monotone"
          dataKey={dataKey as string}
          stroke={color}
          strokeWidth={2}
          dot={false}
          activeDot={{ r: 5, stroke: "var(--background)", strokeWidth: 2 }}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}

export function MetricChart<T extends PointWithTimestamp>(props: Props<T>) {
  const { title, data, serverId } = props;
  const hasData = data.length >= 2;
  const [open, setOpen] = useState(false);
  const [range, setRange] = useState<RangeKey>("1h");
  const extended = isBackendRange(range);

  const { data: extendedData, isFetching: extendedLoading } = useQuery({
    queryKey: ["servers", serverId, "metrics-history", range],
    queryFn: () => api.metricsHistory(serverId!, range),
    enabled: open && extended && !!serverId,
  });

  // extendedData vem de MetricPoint[] (única forma real usada em produção
  // pra esse componente genérico) — cast seguro pro T do chamador, evita
  // duplicar esse componente 4x só pra diferenciar o tipo do dataKey.
  const zoomedData = extended
    ? ((extendedData ?? []) as unknown as T[])
    : filterByRange(data, rangeMs(range), (p) => new Date(p.timestamp).getTime());
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
            <div className="text-muted-foreground flex h-[160px] items-center justify-center text-xs">
              Coletando dados... (amostra a cada 15s)
            </div>
          ) : (
            <Chart {...props} height={160} />
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
              <div className="text-muted-foreground flex h-[320px] items-center justify-center text-xs">
                Carregando histórico...
              </div>
            ) : (
              <Chart {...props} data={zoomedData} height={320} />
            )}
            <p className="text-muted-foreground text-xs">
              {extended
                ? "Dado agregado por hora além das últimas 24h (média/mín/máx) — persistido, sobrevive a reinício do backend."
                : "Histórico em memória (~1h a 15s/amostra) — reseta se o backend reiniciar. O período acima recorta esse buffer, não busca dados mais antigos."}
            </p>
            {gapNote && <p className="text-amber-600 text-xs">{gapNote}</p>}
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
