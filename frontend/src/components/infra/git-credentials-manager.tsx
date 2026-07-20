"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type GitCredentialKind } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { KeyRound, Plus, Trash2 } from "lucide-react";

// Gerenciador de credenciais Git (chave SSH ou PAT) usadas pra clonar
// repositório privado no modo "Git" do fluxo de criar container. Fica num
// dialog próprio em vez de aba dedicada em /infra — uso raro, não merece
// destaque no nível principal da tela.
export function GitCredentialsManager() {
  const [open, setOpen] = useState(false);

  const { data: credentials, isLoading } = useQuery({
    queryKey: ["git-credentials"],
    queryFn: () => api.listGitCredentials(),
    enabled: open,
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["git-credentials"] });

  const [name, setName] = useState("");
  const [kind, setKind] = useState<GitCredentialKind>("ssh_key");
  const [username, setUsername] = useState("");
  const [secret, setSecret] = useState("");

  const create = useMutation({
    mutationFn: () =>
      api.createGitCredential({
        name,
        kind,
        username: kind === "pat" ? username : undefined,
        secret,
      }),
    onSuccess: () => {
      toast.success("Credencial salva");
      setName("");
      setUsername("");
      setSecret("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao salvar credencial"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.removeGitCredential(id),
    onSuccess: () => {
      toast.success("Credencial removida");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover credencial"),
  });

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button type="button" variant="link" size="sm" className="h-auto p-0" />}>
        Gerenciar credenciais
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5">
            <KeyRound className="size-4" />
            Credenciais Git
          </DialogTitle>
        </DialogHeader>

        {isLoading ? (
          <p className="text-muted-foreground text-sm">Carregando...</p>
        ) : !credentials || credentials.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhuma credencial salva.</p>
        ) : (
          <ul className="divide-y rounded-md border">
            {credentials.map((c) => (
              <li key={c.id} className="flex items-center justify-between px-3 py-2 text-sm">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{c.name}</span>
                  <Badge variant="secondary">{c.kind === "ssh_key" ? "Chave SSH" : "PAT"}</Badge>
                  {c.username && <span className="text-muted-foreground text-xs">{c.username}</span>}
                </div>
                <Button
                  size="icon-xs"
                  variant="ghost"
                  className="text-red-600"
                  disabled={remove.isPending}
                  onClick={() => remove.mutate(c.id)}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </li>
            ))}
          </ul>
        )}

        <form
          className="grid gap-2.5 border-t pt-3"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate();
          }}
        >
          <div className="grid grid-cols-2 gap-2.5">
            <div className="grid gap-1.5">
              <Label>Nome</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="minha-conta" />
            </div>
            <div className="grid gap-1.5">
              <Label>Tipo</Label>
              <Select value={kind} onValueChange={(v) => v && setKind(v as GitCredentialKind)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ssh_key">Chave SSH</SelectItem>
                  <SelectItem value="pat">Personal access token</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          {kind === "pat" && (
            <div className="grid gap-1.5">
              <Label>Usuário</Label>
              <Input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="git" />
            </div>
          )}
          <div className="grid gap-1.5">
            <Label>{kind === "ssh_key" ? "Chave privada (PEM)" : "Token"}</Label>
            <textarea
              className="border-input min-h-20 rounded-md border bg-transparent p-2 font-mono text-xs outline-none"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              placeholder={kind === "ssh_key" ? "-----BEGIN OPENSSH PRIVATE KEY-----" : "ghp_..."}
            />
          </div>
          <Button
            type="submit"
            size="sm"
            className="justify-self-start"
            disabled={create.isPending || !name.trim() || !secret.trim()}
          >
            <Plus className="size-4" />
            {create.isPending ? "Salvando..." : "Adicionar credencial"}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
