"use client";

import { useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
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
import { api, type ContainerStat, type PlatformMetricPoint, type PlatformStats } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Cpu, MemoryStick, HardDrive, Network, PlugZap, Disc } from "lucide-react";
import { Sparkline } from "./sparkline";
import { RegisterDialog } from "./discover-servers-dialog";
import { TimeRangeButtons, filterByRange, isBackendRange, rangeMs, type RangeKey } from "./timerange-buttons";

function formatClockTime(ms: number) {
  return new Date(ms).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}

function zip(timestamps: number[], values: number[]) {
  return timestamps.map((timestamp, i) => ({ timestamp, value: values[i] ?? 0 }));
}

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

// Extractors em nível de módulo — reaproveitados tanto pro cálculo inicial
// (janela curta, já em memória) quanto pelo SparkCard quando busca um
// período estendido (24h/7d/30d) do backend, mesma transformação nos dois
// casos.
function extractCPU(h: PlatformMetricPoint[]) {
  return zip(h.map((p) => new Date(p.timestamp).getTime()), h.map((p) => p.cpu_percent));
}
function extractMemory(h: PlatformMetricPoint[]) {
  return zip(h.map((p) => new Date(p.timestamp).getTime()), h.map((p) => p.memory_used_mb));
}
function extractDisk(h: PlatformMetricPoint[]) {
  return zip(h.map((p) => new Date(p.timestamp).getTime()), h.map((p) => p.disk_used_bytes));
}
function extractNetwork(h: PlatformMetricPoint[]) {
  const timestamps = h.map((p) => new Date(p.timestamp).getTime());
  const rx = toRateSeries(h.map((p) => p.network_rx_bytes), timestamps);
  const tx = toRateSeries(h.map((p) => p.network_tx_bytes), timestamps);
  const net = rx.map((v, i) => v + (tx[i] ?? 0));
  return zip(timestamps.slice(1), net);
}

function formatOps(v: number) {
  return `${v.toFixed(1)} op/s`;
}

// Read/write já chegam como taxa pronta do backend (docker.HostIOPS calcula
// por delta lá, ver internal/docker/hostiops.go) — diferente de rede, não
// precisa de toRateSeries aqui.
function extractReadOps(h: PlatformMetricPoint[]) {
  return zip(h.map((p) => new Date(p.timestamp).getTime()), h.map((p) => p.read_ops_per_sec));
}
function extractWriteOps(h: PlatformMetricPoint[]) {
  return zip(h.map((p) => new Date(p.timestamp).getTime()), h.map((p) => p.write_ops_per_sec));
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
  const rxSeries = toRateSeries(
    (history ?? []).map((p) => p.network_rx_bytes),
    timestamps
  );
  const txSeries = toRateSeries(
    (history ?? []).map((p) => p.network_tx_bytes),
    timestamps
  );

  const memPercent =
    data.total_memory_limit_mb > 0 ? (data.total_memory_used_mb / data.total_memory_limit_mb) * 100 : 0;
  const diskPercent = data.disk_total_bytes > 0 ? (data.disk_used_bytes / data.disk_total_bytes) * 100 : 0;
  const currentRxRate = rxSeries.length > 0 ? rxSeries[rxSeries.length - 1] : 0;
  const currentTxRate = txSeries.length > 0 ? txSeries[txSeries.length - 1] : 0;

  const cpuPoints = extractCPU(history ?? []);
  const memPoints = extractMemory(history ?? []);
  const diskPoints = extractDisk(history ?? []);
  const netPoints = extractNetwork(history ?? []);
  const readOpsPoints = extractReadOps(history ?? []);
  const writeOpsPoints = extractWriteOps(history ?? []);
  const currentReadOps = readOpsPoints.length > 0 ? readOpsPoints[readOpsPoints.length - 1].value : 0;
  const currentWriteOps = writeOpsPoints.length > 0 ? writeOpsPoints[writeOpsPoints.length - 1].value : 0;

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-4 gap-4">
        <SparkCard
          icon={<Cpu className="size-4" />}
          label="CPU (todos os containers)"
          value={`${data.total_cpu_percent.toFixed(1)}%`}
          hint={`${data.containers.length} container(s)`}
          points={cpuPoints}
          extract={extractCPU}
          color="#2563eb"
          formatValue={(v) => `${v.toFixed(1)}%`}
        />
        <SparkCard
          icon={<MemoryStick className="size-4" />}
          label="Memória"
          value={`${data.total_memory_used_mb.toFixed(0)} MB`}
          hint={`de ${data.total_memory_limit_mb.toFixed(0)} MB (${memPercent.toFixed(0)}%)`}
          points={memPoints}
          extract={extractMemory}
          color="#7c3aed"
          formatValue={(v) => `${v.toFixed(0)} MB`}
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
          points={diskPoints}
          extract={extractDisk}
          color="#059669"
          formatValue={formatBytes}
        />
        <SparkCard
          icon={<Network className="size-4" />}
          label="Rede"
          value={`↓${formatRate(currentRxRate)}`}
          hint={`↑${formatRate(currentTxRate)}`}
          points={netPoints}
          extract={extractNetwork}
          color="#0891b2"
          formatValue={formatRate}
        />
      </div>

      <IOPSCard
        readPoints={readOpsPoints}
        writePoints={writeOpsPoints}
        currentRead={currentReadOps}
        currentWrite={currentWriteOps}
      />

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
  points,
  extract,
  color,
  formatValue,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint?: string;
  points: { timestamp: number; value: number }[];
  extract: (h: PlatformMetricPoint[]) => { timestamp: number; value: number }[];
  color: string;
  formatValue: (v: number) => string;
}) {
  const [open, setOpen] = useState(false);
  const [range, setRange] = useState<RangeKey>("1h");
  const extended = isBackendRange(range);
  const hasData = points.length >= 2;

  const { data: extendedHistory, isFetching: extendedLoading } = useQuery({
    queryKey: ["platform-stats-history", range],
    queryFn: () => api.platformStatsHistory(range),
    enabled: open && extended,
  });

  const zoomed = extended
    ? extract(extendedHistory ?? [])
    : filterByRange(points, rangeMs(range), (p) => p.timestamp);

  return (
    <>
      <Card
        className={hasData ? "cursor-pointer transition-colors hover:bg-muted/40" : undefined}
        onClick={() => hasData && setOpen(true)}
        title={hasData ? "Clique pra ampliar e mudar o período" : undefined}
      >
        <CardContent className="p-4">
          <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
            {icon}
            {label}
          </p>
          <p className="text-2xl font-semibold">{value}</p>
          {hint && <p className="text-muted-foreground text-xs">{hint}</p>}
          <div className="mt-2 -mb-2 -ml-1">
            <Sparkline data={points.map((p) => p.value)} color={color} />
          </div>
        </CardContent>
      </Card>

      {open && (
        <Dialog open onOpenChange={setOpen}>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>{label}</DialogTitle>
            </DialogHeader>
            <TimeRangeButtons value={range} onChange={setRange} />
            {extended && extendedLoading ? (
              <div className="text-muted-foreground flex h-[320px] items-center justify-center text-xs">
                Carregando histórico...
              </div>
            ) : (
            <ResponsiveContainer width="100%" height={320}>
              <LineChart data={zoomed} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border)" />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={formatClockTime}
                  tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                  axisLine={{ stroke: "var(--border)" }}
                  tickLine={false}
                  minTickGap={40}
                />
                <YAxis
                  tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                  axisLine={false}
                  tickLine={false}
                  width={50}
                />
                <Tooltip
                  labelFormatter={(t) => formatClockTime(Number(t))}
                  formatter={(v) => [formatValue(Number(v)), label]}
                  contentStyle={{
                    fontSize: 12,
                    borderRadius: 8,
                    border: "1px solid var(--border)",
                    background: "var(--popover)",
                  }}
                />
                <Line
                  type="monotone"
                  dataKey="value"
                  stroke={color}
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4 }}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
            )}
            <p className="text-muted-foreground text-xs">
              {extended
                ? "Dado agregado por hora além das últimas 24h (média/mín/máx) — persistido, sobrevive a reinício do backend."
                : "Histórico em memória (~1h a 15s/amostra) — reseta se o backend reiniciar. O período acima recorta esse buffer, não busca dados mais antigos."}
            </p>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}

// IOPSCard mostra operações de leitura/escrita por segundo do disco do HOST
// (não bytes, não soma de container) — pedido explícito comparando com um
// painel Zabbix noutro servidor. Duas linhas (não uma, como os outros
// cards) — leitura e escrita têm perfis bem diferentes (ex: WAL do Postgres
// é escrita pesada, leitura de cache raramente bate no disco).
function IOPSCard({
  readPoints,
  writePoints,
  currentRead,
  currentWrite,
}: {
  readPoints: { timestamp: number; value: number }[];
  writePoints: { timestamp: number; value: number }[];
  currentRead: number;
  currentWrite: number;
}) {
  const [open, setOpen] = useState(false);
  const [range, setRange] = useState<RangeKey>("1h");
  const extended = isBackendRange(range);
  const hasData = readPoints.length >= 2;

  const { data: extendedHistory, isFetching: extendedLoading } = useQuery({
    queryKey: ["platform-stats-history", range],
    queryFn: () => api.platformStatsHistory(range),
    enabled: open && extended,
  });

  const zoomedRead = extended
    ? extractReadOps(extendedHistory ?? [])
    : filterByRange(readPoints, rangeMs(range), (p) => p.timestamp);
  const zoomedWrite = extended
    ? extractWriteOps(extendedHistory ?? [])
    : filterByRange(writePoints, rangeMs(range), (p) => p.timestamp);
  // Mescla os dois pra alimentar um LineChart só, com timestamp comum —
  // read/write são amostrados juntos (mesmo tick), então os arrays sempre
  // têm o mesmo comprimento/timestamps.
  const zoomedData = zoomedRead.map((p, i) => ({
    timestamp: p.timestamp,
    read: p.value,
    write: zoomedWrite[i]?.value ?? 0,
  }));

  return (
    <>
      <Card
        className={hasData ? "cursor-pointer transition-colors hover:bg-muted/40" : undefined}
        onClick={() => hasData && setOpen(true)}
        title={hasData ? "Clique pra ampliar e mudar o período" : undefined}
      >
        <CardHeader>
          <CardTitle className="text-sm font-medium flex items-center gap-1.5">
            <Disc className="size-4" />
            I/O de disco do host (operações/s)
          </CardTitle>
        </CardHeader>
        <CardContent>
          {!hasData ? (
            <div className="text-muted-foreground flex h-[120px] items-center justify-center text-xs">
              Coletando dados... (amostra a cada 15s)
            </div>
          ) : (
            <>
              <p className="text-muted-foreground mb-2 text-xs">
                Leitura: <span className="text-foreground font-medium">{formatOps(currentRead)}</span> · Escrita:{" "}
                <span className="text-foreground font-medium">{formatOps(currentWrite)}</span>
              </p>
              <ResponsiveContainer width="100%" height={120}>
                <LineChart data={zoomedData} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
                  <XAxis dataKey="timestamp" hide />
                  <YAxis hide />
                  <Line type="monotone" dataKey="read" stroke="#2563eb" strokeWidth={2} dot={false} isAnimationActive={false} />
                  <Line type="monotone" dataKey="write" stroke="#d97706" strokeWidth={2} dot={false} isAnimationActive={false} />
                </LineChart>
              </ResponsiveContainer>
            </>
          )}
        </CardContent>
      </Card>

      {open && (
        <Dialog open onOpenChange={setOpen}>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>I/O de disco do host (operações/s)</DialogTitle>
            </DialogHeader>
            <TimeRangeButtons value={range} onChange={setRange} />
            {extended && extendedLoading ? (
              <div className="text-muted-foreground flex h-[320px] items-center justify-center text-xs">
                Carregando histórico...
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={320}>
                <LineChart data={zoomedData} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border)" />
                  <XAxis
                    dataKey="timestamp"
                    tickFormatter={formatClockTime}
                    tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                    axisLine={{ stroke: "var(--border)" }}
                    tickLine={false}
                    minTickGap={40}
                  />
                  <YAxis tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} axisLine={false} tickLine={false} width={50} />
                  <Tooltip
                    labelFormatter={(t) => formatClockTime(Number(t))}
                    formatter={(v, name) => [formatOps(Number(v)), name === "read" ? "Leitura" : "Escrita"]}
                    contentStyle={{
                      fontSize: 12,
                      borderRadius: 8,
                      border: "1px solid var(--border)",
                      background: "var(--popover)",
                    }}
                  />
                  <Legend
                    formatter={(v) => (v === "read" ? "Leitura" : "Escrita")}
                    wrapperStyle={{ fontSize: 12 }}
                  />
                  <Line type="monotone" dataKey="read" stroke="#2563eb" strokeWidth={2} dot={false} activeDot={{ r: 4 }} isAnimationActive={false} />
                  <Line type="monotone" dataKey="write" stroke="#d97706" strokeWidth={2} dot={false} activeDot={{ r: 4 }} isAnimationActive={false} />
                </LineChart>
              </ResponsiveContainer>
            )}
            <p className="text-muted-foreground text-xs">
              {extended
                ? "Dado agregado por hora além das últimas 24h (média) — persistido, sobrevive a reinício do backend."
                : "Histórico em memória (~1h a 15s/amostra) — reseta se o backend reiniciar."}{" "}
              Operações completadas por segundo em /proc/diskstats do host (disco inteiro, não por
              container/partição) — não bytes.
            </p>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
