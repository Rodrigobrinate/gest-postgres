"use client";

import Link from "next/link";
import { ArrowLeft, Boxes } from "lucide-react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ContainersTab } from "@/components/infra/containers-tab";
import { NetworksTab } from "@/components/infra/networks-tab";
import { VolumesTab } from "@/components/infra/volumes-tab";
import { ComposeTab } from "@/components/infra/compose-tab";
import { BuildTab } from "@/components/infra/build-tab";
import { TraefikTab } from "@/components/infra/traefik-tab";
import { FirewallTab } from "@/components/infra/firewall-tab";
import { HostFilesTab } from "@/components/infra/host-files-tab";
import { GitDeploymentsTab } from "@/components/infra/git-deployments-tab";

export default function InfraPage() {
  return (
    <div className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 p-6 sm:p-10">
      <div>
        <Link
          href="/"
          className="text-muted-foreground inline-flex items-center gap-1 text-sm hover:text-foreground"
        >
          <ArrowLeft className="size-4" />
          Servidores
        </Link>
      </div>

      <header className="flex items-center gap-3">
        <div className="bg-primary text-primary-foreground flex size-10 items-center justify-center rounded-lg">
          <Boxes className="size-5" />
        </div>
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Docker</h1>
          <p className="text-muted-foreground text-sm">
            Containers, redes e volumes do host — não só os servidores Postgres gerenciados.
          </p>
        </div>
      </header>

      <Tabs defaultValue="containers">
        <TabsList>
          <TabsTrigger value="containers">Containers</TabsTrigger>
          <TabsTrigger value="networks">Redes</TabsTrigger>
          <TabsTrigger value="volumes">Volumes</TabsTrigger>
          <TabsTrigger value="compose">Stacks</TabsTrigger>
          <TabsTrigger value="build">Build</TabsTrigger>
          <TabsTrigger value="traefik">Traefik</TabsTrigger>
          <TabsTrigger value="firewall">Firewall</TabsTrigger>
          <TabsTrigger value="host-files">Arquivos do host</TabsTrigger>
          <TabsTrigger value="git-deployments">Deploy automático</TabsTrigger>
        </TabsList>

        <TabsContent value="containers" className="pt-4">
          <ContainersTab />
        </TabsContent>
        <TabsContent value="networks" className="pt-4">
          <NetworksTab />
        </TabsContent>
        <TabsContent value="volumes" className="pt-4">
          <VolumesTab />
        </TabsContent>
        <TabsContent value="compose" className="pt-4">
          <ComposeTab />
        </TabsContent>
        <TabsContent value="build" className="pt-4">
          <BuildTab />
        </TabsContent>
        <TabsContent value="traefik" className="pt-4">
          <TraefikTab />
        </TabsContent>
        <TabsContent value="firewall" className="pt-4">
          <FirewallTab />
        </TabsContent>
        <TabsContent value="host-files" className="pt-4">
          <HostFilesTab />
        </TabsContent>
        <TabsContent value="git-deployments" className="pt-4">
          <GitDeploymentsTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
