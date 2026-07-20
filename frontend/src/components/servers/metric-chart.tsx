"use client";

import { useState } from "react";
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
import { TimeRangeButtons, filterByRange } from "./timerange-buttons";

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
          activeDot={{ r: 4 }}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}

export function MetricChart<T extends PointWithTimestamp>(props: Props<T>) {
  const { title, data } = props;
  const hasData = data.length >= 2;
  const [open, setOpen] = useState(false);
  const [rangeMs, setRangeMs] = useState(Infinity);

  const zoomedData = filterByRange(data, rangeMs, (p) => new Date(p.timestamp).getTime());

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
            <TimeRangeButtons value={rangeMs} onChange={setRangeMs} />
            <Chart {...props} data={zoomedData} height={320} />
            <p className="text-muted-foreground text-xs">
              Histórico em memória (~1h a 15s/amostra) — reseta se o backend reiniciar. O período
              acima recorta esse buffer, não busca dados mais antigos.
            </p>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
