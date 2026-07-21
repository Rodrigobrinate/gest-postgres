"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { StatusBadge } from "./status-badge";
import { ServerActions } from "./server-actions";
import { Database } from "lucide-react";

const PRESET_LABEL: Record<string, string> = {
  small: "Pequeno",
  medium: "Médio",
  large: "Grande",
  custom: "Customizado",
};

export function ServerTable() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["servers"],
    queryFn: api.listServers,
  });

  if (isLoading) {
    return <p className="text-muted-foreground p-6 text-sm">Carregando servidores...</p>;
  }

  if (isError) {
    return (
      <p className="p-6 text-sm text-red-600">
        Não foi possível carregar os servidores. A API está no ar?
      </p>
    );
  }

  if (!data || data.length === 0) {
    return (
      <div className="text-muted-foreground flex flex-col items-center gap-2 py-16 text-center text-sm">
        <Database className="size-8" />
        <p>Nenhum servidor ainda. Clique em &ldquo;Criar servidor&rdquo; pra provisionar o primeiro.</p>
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Nome</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Versão</TableHead>
          <TableHead>Recursos</TableHead>
          <TableHead>Porta</TableHead>
          <TableHead>Banco</TableHead>
          <TableHead>Conexões</TableHead>
          <TableHead className="text-right">Ações</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {data.map((server) => (
          <TableRow key={server.id}>
            <TableCell>
              <Link href={`/servers/${server.id}`} className="hover:underline">
                <div className="font-medium">{server.name}</div>
              </Link>
              {server.description && (
                <div className="text-muted-foreground text-xs">{server.description}</div>
              )}
            </TableCell>
            <TableCell>
              <StatusBadge status={server.status} />
            </TableCell>
            <TableCell>PostgreSQL {server.version}</TableCell>
            <TableCell>{PRESET_LABEL[server.preset] ?? server.preset}</TableCell>
            <TableCell className="font-mono text-sm">{server.host_port}</TableCell>
            <TableCell className="font-mono text-sm">{server.database_name}</TableCell>
            <TableCell className="text-muted-foreground font-mono text-sm">
              {server.connection_count != null ? server.connection_count : "—"}
            </TableCell>
            <TableCell className="text-right">
              <ServerActions server={server} />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
