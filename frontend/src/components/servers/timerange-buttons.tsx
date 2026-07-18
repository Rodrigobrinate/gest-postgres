"use client";

import { Button } from "@/components/ui/button";

// Filtra só o que já está no histórico em memória (~1h a 15s/amostra) — não
// existe range maior pra buscar, então as opções são recortes desse mesmo
// buffer, não uma query nova ao backend.
export const TIME_RANGES = [
  { label: "5 min", ms: 5 * 60 * 1000 },
  { label: "15 min", ms: 15 * 60 * 1000 },
  { label: "30 min", ms: 30 * 60 * 1000 },
  { label: "Tudo", ms: Infinity },
] as const;

export function TimeRangeButtons({
  value,
  onChange,
}: {
  value: number;
  onChange: (ms: number) => void;
}) {
  return (
    <div className="flex gap-1.5">
      {TIME_RANGES.map((r) => (
        <Button
          key={r.label}
          size="sm"
          variant={value === r.ms ? "default" : "outline"}
          onClick={() => onChange(r.ms)}
        >
          {r.label}
        </Button>
      ))}
    </div>
  );
}

export function filterByRange<T>(points: T[], rangeMs: number, timestampOf: (p: T) => number): T[] {
  if (!isFinite(rangeMs) || points.length === 0) return points;
  const latest = timestampOf(points[points.length - 1]);
  const cutoff = latest - rangeMs;
  return points.filter((p) => timestampOf(p) >= cutoff);
}
