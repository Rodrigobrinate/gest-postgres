"use client";

import { useQuery } from "@tanstack/react-query";
import { api, type ContainerMetricPoint } from "@/lib/api";
import { MetricChart } from "@/components/servers/metric-chart";

export function StatsTab({ containerId }: { containerId: string }) {
  const { data: history } = useQuery({
    queryKey: ["infra-container-stats-history", containerId],
    queryFn: () => api.containerStatsHistory(containerId),
    refetchInterval: 15_000,
  });

  const points: ContainerMetricPoint[] = history ?? [];

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <MetricChart
        title="CPU (%)"
        data={points}
        dataKey="cpu_percent"
        color="var(--chart-1, #6366f1)"
        unit="%"
      />
      <MetricChart
        title="Memória (MB)"
        data={points}
        dataKey="memory_used_mb"
        color="var(--chart-2, #22c55e)"
        formatValue={(v) => `${v.toFixed(0)} MB`}
      />
      <MetricChart
        title="Rede recebida (bytes, acumulado)"
        data={points}
        dataKey="network_rx_bytes"
        color="var(--chart-3, #f59e0b)"
        formatValue={(v) => `${(v / 1024 / 1024).toFixed(1)} MB`}
      />
      <MetricChart
        title="Rede enviada (bytes, acumulado)"
        data={points}
        dataKey="network_tx_bytes"
        color="var(--chart-4, #ef4444)"
        formatValue={(v) => `${(v / 1024 / 1024).toFixed(1)} MB`}
      />
    </div>
  );
}
