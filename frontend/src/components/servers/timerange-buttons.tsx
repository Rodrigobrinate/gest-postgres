"use client";

import { Button } from "@/components/ui/button";

// "5min"-"1h" recortam o buffer já carregado em memória (client-side, sem ir
// no backend) — "24h"/"7d"/"30d" passam de um dado que só existia em
// memória (~1h) e precisam buscar em metric_history (Postgres), então
// `backend: true` marca isso pros componentes que consomem TIME_RANGES
// saberem quando disparar uma query nova em vez de só filtrar o array que
// já tinham.
export const TIME_RANGES = [
  { key: "5m", label: "5 min", ms: 5 * 60 * 1000, backend: false },
  { key: "15m", label: "15 min", ms: 15 * 60 * 1000, backend: false },
  { key: "30m", label: "30 min", ms: 30 * 60 * 1000, backend: false },
  { key: "1h", label: "1h (tudo em memória)", ms: Infinity, backend: false },
  { key: "24h", label: "24h", ms: Infinity, backend: true },
  { key: "7d", label: "7 dias", ms: Infinity, backend: true },
  { key: "30d", label: "30 dias", ms: Infinity, backend: true },
] as const;

export type RangeKey = (typeof TIME_RANGES)[number]["key"];

export function isBackendRange(key: RangeKey): boolean {
  return TIME_RANGES.find((r) => r.key === key)?.backend ?? false;
}

export function rangeMs(key: RangeKey): number {
  return TIME_RANGES.find((r) => r.key === key)?.ms ?? Infinity;
}

export function TimeRangeButtons({
  value,
  onChange,
}: {
  value: RangeKey;
  onChange: (key: RangeKey) => void;
}) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {TIME_RANGES.map((r) => (
        <Button
          key={r.key}
          size="sm"
          variant={value === r.key ? "default" : "outline"}
          onClick={() => onChange(r.key)}
        >
          {r.label}
        </Button>
      ))}
    </div>
  );
}

export function filterByRange<T>(points: T[], ms: number, timestampOf: (p: T) => number): T[] {
  if (!isFinite(ms) || points.length === 0) return points;
  const latest = timestampOf(points[points.length - 1]);
  const cutoff = latest - ms;
  return points.filter((p) => timestampOf(p) >= cutoff);
}
