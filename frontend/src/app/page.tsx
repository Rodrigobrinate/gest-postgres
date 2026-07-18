import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CreateServerDialog } from "@/components/servers/create-server-dialog";
import { DiscoverServersDialog } from "@/components/servers/discover-servers-dialog";
import { ServerTable } from "@/components/servers/server-table";
import { Database } from "lucide-react";

export default function Home() {
  return (
    <div className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 p-6 sm:p-10">
      <header className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="bg-primary text-primary-foreground flex size-10 items-center justify-center rounded-lg">
            <Database className="size-5" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">gest-postgres</h1>
            <p className="text-muted-foreground text-sm">
              Servidores PostgreSQL gerenciados
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <DiscoverServersDialog />
          <CreateServerDialog />
        </div>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Servidores</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <ServerTable />
        </CardContent>
      </Card>
    </div>
  );
}
