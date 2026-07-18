"use client";

import { useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type ContainerStat, type PlatformStats } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Cpu, MemoryStick, HardDrive, Network, PlugZap } from "lucide-react";
import { Sparkline } from "./sparkline";
import { RegisterDialog } from "./discover-servers-dialog";

function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 4) return `${(bytes / 1024 ** 4).toFixed(2)} TB`;
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(0)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
}

function formatRate(bytesPerSec: number) {
  return `${formatBytes(Math.max(bytesPerSec, 0))}/s`;
}

// Diferença consecutiva vira taxa (bytes/s) — o histórico guarda acumulado,
// igual o docker faz, então uma amostra sozinha não dá "velocidade", precisa
// de duas.
function toRateSeries(values: number[], timestamps: number[]) {
  const rates: number[] = [];
  for (let i = 1; i < values.length; i++) {
    const dt = (timestamps[i] - timestamps[i - 1]) / 1000;
    rates.push(dt > 0 ? Math.max((values[i] - values[i - 1]) / dt, 0) : 0);
  }
  return rates;
}

export function PlatformStatsCards() {
  const queryClient = useQueryClient();
  const [adopting, setAdopting] = useState<ContainerStat | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["platform-stats"],
    queryFn: () => api.platformStats(),
    refetchInterval: 15_000,
  });

  const { data: history } = useQuery({
    queryKey: ["platform-stats-history"],
    queryFn: () => api.platformStatsHistory(),
    refetchInterval: 15_000,
  });

  // Guarda o poll anterior pra comparar "subiu/desceu" — padrão oficial do
  // React pra "state derivado de mudança de prop" (setState direto no corpo
  // do render quando o valor mudou desde o render anterior, não dentro de
  // useEffect — ver "Adjusting some state when a prop changes" nos docs).
  const [lastSeen, setLastSeen] = useState<PlatformStats | undefined>(undefined);
  const [previous, setPrevious] = useState<Map<string, ContainerStat>>(new Map());
  if (data && data !== lastSeen) {
    setLastSeen(data);
    if (lastSeen) {
      const next = new Map<string, ContainerStat>();
      for (const c of lastSeen.containers) next.set(c.container_id, c);
      setPrevious(next);
    }
  }

  if (isLoading || !data) {
    return (
      <div className="grid grid-cols-4 gap-4">
        {["CPU", "Memória", "Disco", "Rede"].map((label) => (
          <Card key={label}>
            <CardContent className="p-4">
              <p className="text-muted-foreground text-xs">{label}</p>
              <p className="text-2xl font-semibold">—</p>
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  const timestamps = (history ?? []).map((p) => new Date(p.timestamp).getTime());
  const cpuSeries = (history ?? []).map((p) => p.cpu_percent);
  const memSeries = (history ?? []).map((p) => p.memory_used_mb);
  const diskSeries = (history ?? []).map((p) => p.disk_used_bytes);
  const rxSeries = toRateSeries(
    (history ?? []).map((p) => p.network_rx_bytes),
    timestamps
  );
  const txSeries = toRateSeries(
    (history ?? []).map((p) => p.network_tx_bytes),
    timestamps
  );
  const netSeries = rxSeries.map((v, i) => v + (txSeries[i] ?? 0));

  const memPercent =
    data.total_memory_limit_mb > 0 ? (data.total_memory_used_mb / data.total_memory_limit_mb) * 100 : 0;
  const diskPercent = data.disk_total_bytes > 0 ? (data.disk_used_bytes / data.disk_total_bytes) * 100 : 0;
  const currentRxRate = rxSeries.length > 0 ? rxSeries[rxSeries.length - 1] : 0;
  const currentTxRate = txSeries.length > 0 ? txSeries[txSeries.length - 1] : 0;

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-4 gap-4">
        <SparkCard
          icon={<Cpu className="size-4" />}
          label="CPU (todos os containers)"
          value={`${data.total_cpu_percent.toFixed(1)}%`}
          hint={`${data.containers.length} container(s)`}
          series={cpuSeries}
          color="#2563eb"
        />
        <SparkCard
          icon={<MemoryStick className="size-4" />}
          label="Memória"
          value={`${data.total_memory_used_mb.toFixed(0)} MB`}
          hint={`de ${data.total_memory_limit_mb.toFixed(0)} MB (${memPercent.toFixed(0)}%)`}
          series={memSeries}
          color="#7c3aed"
        />
        <SparkCard
          icon={<HardDrive className="size-4" />}
          label="Disco"
          value={data.disk_available ? `${diskPercent.toFixed(0)}%` : "—"}
          hint={
            data.disk_available
              ? `${formatBytes(data.disk_used_bytes)} de ${formatBytes(data.disk_total_bytes)}`
              : "mount /hostfs indisponível"
          }
          series={diskSeries}
          color="#059669"
        />
        <SparkCard
          icon={<Network className="size-4" />}
          label="Rede"
          value={`↓${formatRate(currentRxRate)}`}
          hint={`↑${formatRate(currentTxRate)}`}
          series={netSeries}
          color="#0891b2"
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Containers</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {data.containers.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhum container rodando.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-muted-foreground border-b text-xs">
                    <th className="px-4 py-2 text-left font-normal">Nome</th>
                    <th className="px-4 py-2 text-right font-normal">CPU</th>
                    <th className="px-4 py-2 text-right font-normal">Memória</th>
                    <th className="px-4 py-2 text-right font-normal">Peso do container</th>
                    <th className="px-4 py-2 text-right font-normal">I/O disco (leitura/escrita)</th>
                    <th className="px-4 py-2 text-right font-normal">Rede (acumulado)</th>
                  </tr>
                </thead>
                <tbody>
                  {data.containers.map((c) => {
                    const prev = previous.get(c.container_id);
                    return (
                      <tr
                        key={c.container_id}
                        className={
                          c.adoptable
                            ? "hover:bg-muted/50 border-b last:border-0 cursor-pointer"
                            : "border-b last:border-0"
                        }
                        onClick={() => c.adoptable && setAdopting(c)}
                        title={c.adoptable ? "Clique pra tornar esse container um servidor gerenciado" : undefined}
                      >
                        <td className="px-4 py-2">
                          <div className="flex items-center gap-2">
                            <span className="truncate font-mono">{c.server_name ?? c.name}</span>
                            {c.is_managed && <Badge variant="outline">gerenciado</Badge>}
                            {c.adoptable && (
                              <Badge variant="outline" className="border-blue-200 bg-blue-50 text-blue-700">
                                <PlugZap className="size-3" />
                                adotar
                              </Badge>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-2 text-right font-mono text-xs">
                          <Trend value={c.cpu_percent} previous={prev?.cpu_percent}>
                            {c.cpu_percent.toFixed(1)}%
                          </Trend>
                        </td>
                        <td className="px-4 py-2 text-right font-mono text-xs">
                          <Trend value={c.memory_used_mb} previous={prev?.memory_used_mb}>
                            {c.memory_used_mb.toFixed(0)} MB
                          </Trend>
                        </td>
                        <td className="px-4 py-2 text-right font-mono text-xs">
                          {c.volume_size_bytes != null ? formatBytes(c.volume_size_bytes) : "—"}
                        </td>
                        <td className="text-muted-foreground px-4 py-2 text-right font-mono text-xs">
                          ↓{formatBytes(c.block_read_bytes)} ↑{formatBytes(c.block_write_bytes)}
                          {(c.block_read_ops > 0 || c.block_write_ops > 0) && (
                            <span> · {c.block_read_ops + c.block_write_ops} ops</span>
                          )}
                        </td>
                        <td className="text-muted-foreground px-4 py-2 text-right font-mono text-xs">
                          ↓{formatBytes(c.network_rx_bytes)} ↑{formatBytes(c.network_tx_bytes)}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {adopting && (
        <RegisterDialog
          container={{ container_id: adopting.container_id, name: adopting.server_name ?? adopting.name }}
          onClose={() => setAdopting(null)}
          onRegistered={() => {
            setAdopting(null);
            queryClient.invalidateQueries({ queryKey: ["servers"] });
            queryClient.invalidateQueries({ queryKey: ["platform-stats"] });
          }}
        />
      )}
    </div>
  );
}

// Vermelho se subiu em relação à amostra anterior, verde se desceu — mesma
// lógica de ticker de mercado financeiro, aplicada às métricas "ao vivo"
// (CPU/memória) que fazem sentido comparar ponto a ponto.
function Trend({
  value,
  previous,
  children,
}: {
  value: number;
  previous?: number;
  children: ReactNode;
}) {
  let color = "";
  if (previous != null && Math.abs(value - previous) > 0.05) {
    color = value > previous ? "text-red-600" : "text-emerald-600";
  }
  return <span className={color}>{children}</span>;
}

function SparkCard({
  icon,
  label,
  value,
  hint,
  series,
  color,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint?: string;
  series: number[];
  color: string;
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
          {icon}
          {label}
        </p>
        <p className="text-2xl font-semibold">{value}</p>
        {hint && <p className="text-muted-foreground text-xs">{hint}</p>}
        <div className="mt-2 -mb-2 -ml-1">
          <Sparkline data={series} color={color} />
        </div>
      </CardContent>
    </Card>
  );
}
