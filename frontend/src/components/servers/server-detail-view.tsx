"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import { StatusBadge } from "./status-badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ArrowLeft, Database } from "lucide-react";
import { MonitoringTab } from "./tabs/monitoring-tab";
import { LogsTab } from "./tabs/logs-tab";
import { SqlEditorTab } from "./tabs/sql-editor-tab";
import { TablesTab } from "./tabs/tables-tab";

export function ServerDetailView({ id }: { id: string }) {
  const { data: server, isLoading } = useQuery({
    queryKey: ["servers", id],
    queryFn: () => api.getServer(id),
  });

  const { data: databases } = useQuery({
    queryKey: ["servers", id, "databases"],
    queryFn: () => api.listDatabases(id),
    enabled: server?.status === "running",
    refetchInterval: false,
  });

  const [database, setDatabase] = useState<string | null>(null);
  const activeDatabase = database ?? server?.database_name ?? "";

  if (isLoading) {
    return <div className="text-muted-foreground p-10 text-sm">Carregando...</div>;
  }
  if (!server) {
    return <div className="p-10 text-sm text-red-600">Servidor não encontrado.</div>;
  }

  const notRunning = server.status !== "running";

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 p-6 sm:p-10">
      <div>
        <Link
          href="/"
          className="text-muted-foreground inline-flex items-center gap-1 text-sm hover:text-foreground"
        >
          <ArrowLeft className="size-4" />
          Servidores
        </Link>
      </div>

      <header className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="bg-primary text-primary-foreground flex size-10 items-center justify-center rounded-lg">
            <Database className="size-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-semibold tracking-tight">{server.name}</h1>
              <StatusBadge status={server.status} />
            </div>
            <p className="text-muted-foreground text-sm">
              PostgreSQL {server.version} · porta {server.host_port} · {server.container_name}
            </p>
          </div>
        </div>

        {databases && databases.length > 0 && (
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground text-sm">Banco:</span>
            <Select value={activeDatabase} onValueChange={(v) => v && setDatabase(v)}>
              <SelectTrigger className="w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {databases.map((d) => (
                  <SelectItem key={d} value={d}>
                    {d}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
      </header>

      {notRunning ? (
        <div className="text-muted-foreground rounded-lg border bg-card p-10 text-center text-sm">
          Servidor precisa estar rodando pra ver monitoramento, logs, editor SQL e tabelas.
          {server.status === "creating" && " Ainda provisionando — atualiza em alguns segundos."}
        </div>
      ) : (
        <Tabs defaultValue="monitoring">
          <TabsList>
            <TabsTrigger value="monitoring">Monitoramento</TabsTrigger>
            <TabsTrigger value="logs">Logs</TabsTrigger>
            <TabsTrigger value="sql">Editor SQL</TabsTrigger>
            <TabsTrigger value="tables">Tabelas</TabsTrigger>
          </TabsList>

          <TabsContent value="monitoring" className="pt-4">
            <MonitoringTab serverId={id} database={activeDatabase} />
          </TabsContent>
          <TabsContent value="logs" className="pt-4">
            <LogsTab serverId={id} />
          </TabsContent>
          <TabsContent value="sql" className="pt-4">
            <SqlEditorTab serverId={id} database={activeDatabase} />
          </TabsContent>
          <TabsContent value="tables" className="pt-4">
            <TablesTab serverId={id} database={activeDatabase} />
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}
