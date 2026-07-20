"use client";

import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type NetworkEndpoint } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Network, Plus, Trash2 } from "lucide-react";

export function NetworksTab({
  containerId,
  networks,
  onChanged,
}: {
  containerId: string;
  networks: Record<string, NetworkEndpoint>;
  onChanged: () => void;
}) {
  const [selected, setSelected] = useState("");

  const { data: allNetworks } = useQuery({
    queryKey: ["infra-networks"],
    queryFn: () => api.listInfraNetworks(),
  });

  const connect = useMutation({
    mutationFn: () => api.connectContainerNetwork(containerId, selected),
    onSuccess: () => {
      toast.success("Container conectado à rede");
      setSelected("");
      onChanged();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao conectar"),
  });

  const disconnect = useMutation({
    mutationFn: (name: string) => api.disconnectContainerNetwork(containerId, name),
    onSuccess: () => {
      toast.success("Container desconectado da rede");
      onChanged();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao desconectar"),
  });

  const connectedNames = new Set(Object.keys(networks));
  const availableToConnect = (allNetworks ?? []).filter((n) => !connectedNames.has(n.name));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <Network className="size-4" />
          Redes
        </CardTitle>
        <div className="flex items-center gap-1.5">
          <Select value={selected} onValueChange={(v) => setSelected(v ?? "")}>
            <SelectTrigger className="w-48">
              <SelectValue placeholder="Escolher rede" />
            </SelectTrigger>
            <SelectContent>
              {availableToConnect.map((n) => (
                <SelectItem key={n.id} value={n.name}>
                  {n.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button size="sm" disabled={!selected || connect.isPending} onClick={() => connect.mutate()}>
            <Plus className="size-4" />
            Conectar
          </Button>
        </div>
      </CardHeader>
      <CardContent className="p-0">
        {Object.keys(networks).length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Não conectado a nenhuma rede.</p>
        ) : (
          <ul className="divide-y">
            {Object.entries(networks).map(([name, ep]) => (
              <li key={name} className="flex items-center justify-between px-4 py-2 text-sm">
                <div>
                  <span className="font-mono font-medium">{name}</span>
                  <span className="text-muted-foreground ml-2 font-mono text-xs">
                    {ep.ip_address || "sem IP"}
                  </span>
                </div>
                <Button
                  size="icon-xs"
                  variant="ghost"
                  className="text-red-600"
                  disabled={disconnect.isPending}
                  onClick={() => disconnect.mutate(name)}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
