"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type DiscoveredContainer } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Search, RefreshCw, Boxes } from "lucide-react";

export function DiscoverServersDialog() {
  const [open, setOpen] = useState(false);
  const [registering, setRegistering] = useState<DiscoveredContainer | null>(null);

  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["discover"],
    queryFn: () => api.discover(),
    enabled: open,
  });

  const queryClient = useQueryClient();

  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger render={<Button variant="outline" />}>
          <Search className="size-4" />
          Procurar servidores
        </DialogTrigger>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Procurar servidores Postgres</DialogTitle>
            <DialogDescription>
              Containers Docker no host que parecem rodar Postgres e ainda não estão
              cadastrados na plataforma. Não detecta Postgres instalado fora de Docker.
            </DialogDescription>
          </DialogHeader>

          <div className="flex justify-end">
            <Button size="sm" variant="outline" onClick={() => refetch()} disabled={isFetching}>
              <RefreshCw className={isFetching ? "size-3.5 animate-spin" : "size-3.5"} />
              Atualizar
            </Button>
          </div>

          {isLoading ? (
            <p className="text-muted-foreground p-6 text-center text-sm">Procurando...</p>
          ) : !data || data.length === 0 ? (
            <div className="text-muted-foreground flex flex-col items-center gap-2 p-10 text-center text-sm">
              <Boxes className="size-8" />
              Nenhum container novo encontrado.
            </div>
          ) : (
            <ul className="divide-y rounded-md border">
              {data.map((c) => (
                <li key={c.container_id} className="flex items-center justify-between gap-3 p-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-medium">{c.name}</span>
                      <Badge variant="outline">{c.state}</Badge>
                    </div>
                    <p className="text-muted-foreground truncate text-xs">
                      {c.image} {c.ports.length > 0 && `· ${c.ports.join(", ")}`}
                    </p>
                  </div>
                  <Button size="sm" onClick={() => setRegistering(c)}>
                    Cadastrar
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </DialogContent>
      </Dialog>

      {registering && (
        <RegisterDialog
          container={registering}
          onClose={() => setRegistering(null)}
          onRegistered={() => {
            setRegistering(null);
            setOpen(false);
            queryClient.invalidateQueries({ queryKey: ["servers"] });
          }}
        />
      )}
    </>
  );
}

export function RegisterDialog({
  container,
  onClose,
  onRegistered,
}: {
  container: Pick<DiscoveredContainer, "container_id" | "name">;
  onClose: () => void;
  onRegistered: () => void;
}) {
  const [name, setName] = useState(container.name);
  const [username, setUsername] = useState("postgres");
  const [password, setPassword] = useState("");
  const [databaseName, setDatabaseName] = useState("postgres");

  const register = useMutation({
    mutationFn: () =>
      api.registerDiscovered(container.container_id, {
        name,
        username,
        password,
        database_name: databaseName,
      }),
    onSuccess: (server) => {
      toast.success(`"${server.name}" cadastrado`);
      onRegistered();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao cadastrar"),
  });

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Cadastrar &ldquo;{container.name}&rdquo;</DialogTitle>
          <DialogDescription>
            Credenciais reais desse Postgres — a plataforma testa a conexão antes de
            salvar. Nada é criado, só passa a gerenciar o que já existe.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3 py-2">
          <div className="grid gap-1.5">
            <Label>Nome na plataforma</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label>Usuário</Label>
              <Input value={username} onChange={(e) => setUsername(e.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label>Banco</Label>
              <Input value={databaseName} onChange={(e) => setDatabaseName(e.target.value)} />
            </div>
          </div>
          <div className="grid gap-1.5">
            <Label>Senha</Label>
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button
            disabled={register.isPending || !name.trim() || !username.trim() || !password || !databaseName.trim()}
            onClick={() => register.mutate()}
          >
            {register.isPending ? "Validando conexão..." : "Cadastrar"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
