"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Network, Plus, Trash2 } from "lucide-react";

const PROTECTED = new Set(["bridge", "host", "none", "gestpg-internal", "gestpg-managed"]);

export function NetworksTab() {
  const { data: networks, isLoading } = useQuery({
    queryKey: ["infra-networks"],
    queryFn: () => api.listInfraNetworks(),
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["infra-networks"] });

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");

  const create = useMutation({
    mutationFn: () => api.createInfraNetwork(name),
    onSuccess: () => {
      toast.success("Rede criada");
      setOpen(false);
      setName("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar rede"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.removeInfraNetwork(id),
    onSuccess: () => {
      toast.success("Rede removida");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover rede"),
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base flex items-center gap-1.5">
          <Network className="size-4" />
          Redes
        </CardTitle>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" />}>
            <Plus className="size-4" />
            Nova rede
          </DialogTrigger>
          <DialogContent className="sm:max-w-sm">
            <DialogHeader>
              <DialogTitle>Nova rede</DialogTitle>
            </DialogHeader>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="minha-rede" />
            <DialogFooter>
              <Button disabled={create.isPending || !name.trim()} onClick={() => create.mutate()}>
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
        ) : !networks || networks.length === 0 ? (
          <p className="text-muted-foreground p-6 text-sm">Nenhuma rede.</p>
        ) : (
          <ul className="divide-y">
            {networks.map((n) => (
              <li key={n.id} className="flex items-center justify-between px-4 py-2 text-sm">
                <div className="flex items-center gap-2">
                  <span className="font-mono">{n.name}</span>
                  <span className="text-muted-foreground text-xs">{n.driver}</span>
                </div>
                {!PROTECTED.has(n.name) && (
                  <Button
                    size="icon"
                    variant="ghost"
                    className="text-red-600"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(n.id)}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
