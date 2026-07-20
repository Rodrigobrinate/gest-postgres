"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, type NotificationChannelKind } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Bell, Plus, Send, Trash2 } from "lucide-react";

// Tela de configuração de canais de notificação (Telegram / webhook) —
// cadastrados uma vez aqui, referenciados por qualquer regra de alerta de
// qualquer servidor (ver alert-rules.tsx) em vez de colar a mesma
// URL/token em cada regra separadamente.
export function NotificationChannelsManager() {
  const [open, setOpen] = useState(false);

  const { data: channels, isLoading } = useQuery({
    queryKey: ["notification-channels"],
    queryFn: () => api.listNotificationChannels(),
    enabled: open,
  });

  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["notification-channels"] });

  const [name, setName] = useState("");
  const [kind, setKind] = useState<NotificationChannelKind>("telegram");
  const [webhookUrl, setWebhookUrl] = useState("");
  const [botToken, setBotToken] = useState("");
  const [chatId, setChatId] = useState("");

  const create = useMutation({
    mutationFn: () =>
      api.createNotificationChannel({
        name,
        kind,
        webhook_url: kind === "webhook" ? webhookUrl : undefined,
        telegram_bot_token: kind === "telegram" ? botToken : undefined,
        telegram_chat_id: kind === "telegram" ? chatId : undefined,
      }),
    onSuccess: () => {
      toast.success("Canal criado");
      setName("");
      setWebhookUrl("");
      setBotToken("");
      setChatId("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar canal"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteNotificationChannel(id),
    onSuccess: () => {
      toast.success("Canal removido");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover canal"),
  });

  const test = useMutation({
    mutationFn: (id: string) => api.testNotificationChannel(id),
    onSuccess: () => toast.success("Mensagem de teste enviada"),
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao enviar teste — confere token/chat ID/URL"),
  });

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="icon" title="Notificações" />}>
        <Bell className="size-4" />
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5">
            <Bell className="size-4" />
            Canais de notificação
          </DialogTitle>
        </DialogHeader>

        {isLoading ? (
          <p className="text-muted-foreground text-sm">Carregando...</p>
        ) : !channels || channels.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhum canal cadastrado.</p>
        ) : (
          <ul className="divide-y rounded-md border">
            {channels.map((c) => (
              <li key={c.id} className="flex items-center justify-between px-3 py-2 text-sm">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{c.name}</span>
                  <Badge variant="secondary">{c.kind === "telegram" ? "Telegram" : "Webhook"}</Badge>
                </div>
                <div className="flex items-center gap-1">
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    title="Testar"
                    disabled={test.isPending}
                    onClick={() => test.mutate(c.id)}
                  >
                    <Send className="size-3.5" />
                  </Button>
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    className="text-red-600"
                    title="Excluir"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(c.id)}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
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
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="ops-telegram" />
            </div>
            <div className="grid gap-1.5">
              <Label>Tipo</Label>
              <Select value={kind} onValueChange={(v) => v && setKind(v as NotificationChannelKind)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="telegram">Telegram</SelectItem>
                  <SelectItem value="webhook">Webhook</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {kind === "telegram" ? (
            <>
              <div className="grid gap-1.5">
                <Label>Bot token</Label>
                <Input
                  value={botToken}
                  onChange={(e) => setBotToken(e.target.value)}
                  placeholder="123456:ABC-DEF..."
                  className="font-mono text-xs"
                />
              </div>
              <div className="grid gap-1.5">
                <Label>Chat ID</Label>
                <Input value={chatId} onChange={(e) => setChatId(e.target.value)} placeholder="-1001234567890" />
              </div>
              <p className="text-muted-foreground text-xs">
                Cria o bot com @BotFather, manda uma mensagem pra ele (ou adiciona no grupo) e pega o
                chat ID em <code>api.telegram.org/bot&lt;TOKEN&gt;/getUpdates</code>.
              </p>
            </>
          ) : (
            <div className="grid gap-1.5">
              <Label>Webhook URL</Label>
              <Input
                value={webhookUrl}
                onChange={(e) => setWebhookUrl(e.target.value)}
                placeholder="https://hooks.slack.com/... ou qualquer endpoint que aceite POST JSON"
              />
            </div>
          )}

          <Button
            type="submit"
            size="sm"
            className="justify-self-start"
            disabled={
              create.isPending ||
              !name.trim() ||
              (kind === "telegram" ? !botToken.trim() || !chatId.trim() : !webhookUrl.trim())
            }
          >
            <Plus className="size-4" />
            {create.isPending ? "Salvando..." : "Adicionar canal"}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
