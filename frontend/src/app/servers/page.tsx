"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { ServerDetailView } from "@/components/servers/server-detail-view";

// Query string (?id=), não path segment ([id]) — output "export" (Cloudflare
// Pages) não consegue gerar 1 HTML por servidor em build time (a lista de
// servidores só existe em runtime, num banco que o build nem alcança).
// generateStaticParams() vazio pareceria funcionar mas na verdade não gera
// HTML nenhum pra rota — refresh/link direto em /servers/{id} vira 404 (doc
// oficial do Next: "Dynamic Routes without generateStaticParams()" é
// explicitamente não suportado em export estático). Query string em cima de
// uma página ÚNICA e estática (/servers.html) é o padrão documentado do
// próprio Next pra SPA com "id" só conhecido no navegador.
function ServerDetailPageInner() {
  const searchParams = useSearchParams();
  const id = searchParams.get("id") ?? "";
  return <ServerDetailView id={id} />;
}

export default function ServerDetailPage() {
  return (
    <Suspense>
      <ServerDetailPageInner />
    </Suspense>
  );
}
