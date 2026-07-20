import { Card, CardContent } from "@/components/ui/card";

export function EnvTab({ env }: { env: string[] }) {
  if (!env || env.length === 0) {
    return <p className="text-muted-foreground text-sm">Sem variáveis de ambiente.</p>;
  }
  return (
    <Card>
      <CardContent className="p-0">
        <ul className="divide-y">
          {env.map((line) => {
            const idx = line.indexOf("=");
            const key = idx >= 0 ? line.slice(0, idx) : line;
            const value = idx >= 0 ? line.slice(idx + 1) : "";
            return (
              <li key={line} className="flex gap-2 px-4 py-2 font-mono text-xs">
                <span className="font-medium">{key}</span>
                <span className="text-muted-foreground truncate">={value}</span>
              </li>
            );
          })}
        </ul>
      </CardContent>
    </Card>
  );
}
