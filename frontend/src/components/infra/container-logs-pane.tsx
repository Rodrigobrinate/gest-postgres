"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

// Corpo da visualização de logs, sem moldura — usado tanto pelo modal
// (containers-tab.tsx LogsDialog) quanto pela aba inline da página de
// detalhe do container, pra não duplicar a lógica de polling.
export function ContainerLogsPane({ containerId, className }: { containerId: string; className?: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["infra-container-logs", containerId],
    queryFn: () => api.infraContainerLogs(containerId, 500),
    refetchInterval: 3_000,
  });

  return (
    <pre className={className ?? "bg-muted max-h-[60vh] overflow-auto rounded-md p-3 text-xs"}>
      {isLoading ? "Carregando..." : data?.logs || "Sem logs."}
    </pre>
  );
}
