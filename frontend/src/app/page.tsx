"use client";

import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { CreateServerDialog } from "@/components/servers/create-server-dialog";
import { DiscoverServersDialog } from "@/components/servers/discover-servers-dialog";
import { PlatformStatsCards } from "@/components/servers/platform-stats-cards";
import { ServerTable } from "@/components/servers/server-table";
import { LogoutButton } from "@/components/auth/logout-button";
import { UsersManager } from "@/components/auth/users-manager";
import { NotificationChannelsManager } from "@/components/servers/notification-channels-manager";
import { UpdateChecker } from "@/components/servers/update-checker";
import { Boxes, Database, ArrowLeftRight } from "lucide-react";
import { MULTI_SERVER_MODE } from "@/lib/multi-server";
import { useSelectedServer } from "@/lib/server-context";
import { InstallationsOverview } from "@/components/master/installations-overview";

export default function Home() {
  // Só em MULTI_SERVER_MODE (build pro Cloudflare Pages) esse hook importa
  // pra alguma coisa — fora dele, selectedServer fica sempre null e a
  // tela de baixo (dashboard de sempre) renderiza direto, sem overview
  // nenhuma no meio. Comportamento de build normal (1 frontend por
  // droplet) inteiramente inalterado.
  const { selectedServer, selectServer } = useSelectedServer();

  if (MULTI_SERVER_MODE && !selectedServer) {
    return <InstallationsOverview />;
  }

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 p-6 sm:p-10">
      <header className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="bg-primary text-primary-foreground flex size-10 items-center justify-center rounded-lg">
            <Database className="size-5" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">
              gest-postgres
              {MULTI_SERVER_MODE && selectedServer?.name && (
                <span className="text-muted-foreground font-normal"> — {selectedServer.name}</span>
              )}
            </h1>
            <p className="text-muted-foreground text-sm">
              Servidores PostgreSQL gerenciados
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {MULTI_SERVER_MODE && (
            <Button variant="outline" onClick={() => selectServer(null)}>
              <ArrowLeftRight className="size-4" />
              Trocar instalação
            </Button>
          )}
          <Button variant="outline" render={<Link href="/infra" />}>
            <Boxes className="size-4" />
            Docker
          </Button>
          <DiscoverServersDialog />
          <CreateServerDialog />
          <UsersManager />
          <NotificationChannelsManager />
          <UpdateChecker />
          <LogoutButton />
        </div>
      </header>

      <PlatformStatsCards />

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
