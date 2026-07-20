import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { ContainerDetail } from "@/lib/api";

function formatDate(iso?: string) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("pt-BR");
}

export function OverviewTab({ detail }: { detail: ContainerDetail }) {
  const labelEntries = Object.entries(detail.labels ?? {});

  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Detalhes</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <div className="text-muted-foreground">ID</div>
          <div className="truncate font-mono text-xs">{detail.id}</div>
          <div className="text-muted-foreground">Imagem</div>
          <div className="font-mono text-xs">{detail.image}</div>
          <div className="text-muted-foreground">Criado em</div>
          <div>{formatDate(detail.created_at)}</div>
          <div className="text-muted-foreground">Iniciado em</div>
          <div>{formatDate(detail.started_at)}</div>
          {!detail.running && (
            <>
              <div className="text-muted-foreground">Finalizado em</div>
              <div>{formatDate(detail.finished_at)}</div>
              <div className="text-muted-foreground">Exit code</div>
              <div>{detail.exit_code}</div>
            </>
          )}
          <div className="text-muted-foreground">Restart policy</div>
          <div className="font-mono text-xs">{detail.restart_policy || "—"}</div>
          <div className="text-muted-foreground">Comando</div>
          <div className="font-mono text-xs">{detail.command?.join(" ") || "—"}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Labels</CardTitle>
        </CardHeader>
        <CardContent>
          {labelEntries.length === 0 ? (
            <p className="text-muted-foreground text-sm">Sem labels.</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {labelEntries.map(([k, v]) => (
                <Badge key={k} variant="outline" className="font-mono text-xs">
                  {k}={v}
                </Badge>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
