import { Badge } from "@/components/ui/badge";
import type { ServerStatus } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Loader2 } from "lucide-react";

const STATUS_LABEL: Record<ServerStatus, string> = {
  creating: "Criando",
  running: "Rodando",
  stopped: "Parado",
  restarting: "Reiniciando",
  error: "Erro",
  removing: "Removendo",
};

const STATUS_CLASS: Record<ServerStatus, string> = {
  creating: "bg-blue-100 text-blue-700 border-blue-200",
  running: "bg-emerald-100 text-emerald-700 border-emerald-200",
  stopped: "bg-zinc-100 text-zinc-700 border-zinc-200",
  restarting: "bg-amber-100 text-amber-700 border-amber-200",
  error: "bg-red-100 text-red-700 border-red-200",
  removing: "bg-zinc-100 text-zinc-500 border-zinc-200",
};

export function StatusBadge({ status }: { status: ServerStatus }) {
  return (
    <Badge variant="outline" className={cn("font-medium gap-1", STATUS_CLASS[status])}>
      {status === "creating" && <Loader2 className="size-3 animate-spin" />}
      {STATUS_LABEL[status] ?? status}
    </Badge>
  );
}
