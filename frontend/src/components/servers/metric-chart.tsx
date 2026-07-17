"use client";

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
import type { MetricPoint } from "@/lib/api";

function formatTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}

export function MetricChart({
  title,
  data,
  dataKey,
  color,
  unit,
  formatValue,
}: {
  title: string;
  data: MetricPoint[];
  dataKey: keyof MetricPoint;
  color: string;
  unit?: string;
  formatValue?: (v: number) => string;
}) {
  const hasData = data.length >= 2;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {!hasData ? (
          <div className="text-muted-foreground flex h-[160px] items-center justify-center text-xs">
            Coletando dados... (amostra a cada 15s)
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={160}>
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
                dataKey={dataKey}
                stroke={color}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4 }}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
