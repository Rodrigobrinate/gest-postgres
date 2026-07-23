"use client";

import { Button } from "@/components/ui/button";

// "5min"-"1h" recortam o buffer já carregado em memória (client-side, sem ir
// no backend) — "24h"/"7d"/"30d" passam de um dado que só existia em
// memória (~1h) e precisam buscar em metric_history (Postgres), então
// `backend: true` marca isso pros componentes que consomem TIME_RANGES
// saberem quando disparar uma query nova em vez de só filtrar o array que
// já tinham.
// durationMs é a duração REAL de cada período (usada só pra calcular se o
// dado devolvido cobre o período inteiro, ver coverageGapNote abaixo) —
// separada de `ms` (usada por filterByRange no recorte local, Infinity de
// propósito nos períodos "backend" porque ali quem decide a janela é a
// query no servidor, não um filtro em cima do array já carregado).
export const TIME_RANGES = [
  { key: "5m", label: "5 min", ms: 5 * 60 * 1000, durationMs: 5 * 60 * 1000, backend: false },
  { key: "15m", label: "15 min", ms: 15 * 60 * 1000, durationMs: 15 * 60 * 1000, backend: false },
  { key: "30m", label: "30 min", ms: 30 * 60 * 1000, durationMs: 30 * 60 * 1000, backend: false },
  { key: "1h", label: "1h (tudo em memória)", ms: Infinity, durationMs: 60 * 60 * 1000, backend: false },
  { key: "24h", label: "24h", ms: Infinity, durationMs: 24 * 60 * 60 * 1000, backend: true },
  { key: "7d", label: "7 dias", ms: Infinity, durationMs: 7 * 24 * 60 * 60 * 1000, backend: true },
  { key: "30d", label: "30 dias", ms: Infinity, durationMs: 30 * 24 * 60 * 60 * 1000, backend: true },
] as const;

export type RangeKey = (typeof TIME_RANGES)[number]["key"];

export function isBackendRange(key: RangeKey): boolean {
  return TIME_RANGES.find((r) => r.key === key)?.backend ?? false;
}

export function rangeMs(key: RangeKey): number {
  return TIME_RANGES.find((r) => r.key === key)?.ms ?? Infinity;
}

function rangeDurationMs(key: RangeKey): number {
  return TIME_RANGES.find((r) => r.key === key)?.durationMs ?? 0;
}

function rangeLabel(key: RangeKey): string {
  return TIME_RANGES.find((r) => r.key === key)?.label ?? key;
}

// coverageGapNote — quando o período pedido é "backend" (24h/7d/30d) mas o
// dado persistido devolvido ainda não cobre a janela inteira (achado ao
// vivo, 2026-07-23: usuário reportou "seletor de período não funciona,
// sempre mostra só ~1h" — na real o backend respondia certo, só que
// `metric_history` ainda não tinha 24h de dado acumulado ainda, e o
// gráfico ficava com a MESMA cara de "1h" sem avisar por quê). Comparado
// contra o próprio dado devolvido (mais antigo x mais recente), nunca
// contra o relógio local — evita ler Date.now() durante o render.
export function coverageGapNote(
  oldest: string | number | undefined,
  newest: string | number | undefined,
  key: RangeKey
): string | null {
  if (oldest === undefined || newest === undefined) return null;
  const expectedMs = rangeDurationMs(key);
  if (!expectedMs) return null;
  const coveredMs = new Date(newest).getTime() - new Date(oldest).getTime();
  if (coveredMs >= expectedMs * 0.9) return null;
  const oldestLabel = new Date(oldest).toLocaleString("pt-BR");
  return `Histórico persistido só começa em ${oldestLabel} — ainda não cobre ${rangeLabel(key)} inteiro (a coleta continua, isso se resolve sozinho com o tempo).`;
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
