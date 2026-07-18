"use client";

import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Cpu, MemoryStick, HardDrive, Network } from "lucide-react";
import { cn } from "@/lib/utils";

function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(0)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
}

export function PlatformStatsCards() {
  const { data, isLoading } = useQuery({
    queryKey: ["platform-stats"],
    queryFn: () => api.platformStats(),
    refetchInterval: 15_000,
  });

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

  const memPercent =
    data.total_memory_limit_mb > 0 ? (data.total_memory_used_mb / data.total_memory_limit_mb) * 100 : 0;

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          icon={<Cpu className="size-4" />}
          label="CPU (todos os containers)"
          value={`${data.total_cpu_percent.toFixed(1)}%`}
        />
        <StatCard
          icon={<MemoryStick className="size-4" />}
          label="Memória"
          value={`${data.total_memory_used_mb.toFixed(0)} MB`}
          hint={`de ${data.total_memory_limit_mb.toFixed(0)} MB (${memPercent.toFixed(0)}%)`}
        />
        <StatCard
          icon={<HardDrive className="size-4" />}
          label="Disco (Docker)"
          value={formatBytes(data.disk_used_bytes)}
          hint="imagens + containers + volumes"
        />
        <StatCard
          icon={<Network className="size-4" />}
          label="Rede (acumulado)"
          value={`↓${formatBytes(data.network_rx_bytes_total)}`}
          hint={`↑${formatBytes(data.network_tx_bytes_total)}`}
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
            <ul className="divide-y">
              {data.containers.map((c) => (
                <li key={c.container_id} className="flex items-center justify-between gap-3 px-4 py-2 text-sm">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="truncate font-mono">{c.server_name ?? c.name}</span>
                    {c.is_managed && <Badge variant="outline">gerenciado</Badge>}
                  </div>
                  <div className="text-muted-foreground flex shrink-0 gap-4 font-mono text-xs">
                    <span className={cn(c.cpu_percent >= 70 && "text-red-600")}>
                      CPU {c.cpu_percent.toFixed(1)}%
                    </span>
                    <span>
                      {c.memory_used_mb.toFixed(0)}/{c.memory_limit_mb.toFixed(0)} MB
                    </span>
                    <span>
                      ↓{formatBytes(c.network_rx_bytes)} ↑{formatBytes(c.network_tx_bytes)}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
  hint,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint?: string;
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
      </CardContent>
    </Card>
  );
}
