"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { ContainerDetailView } from "@/components/infra/container-detail/container-detail-view";

// Query string (?id=), mesmo raciocínio de app/servers/page.tsx — ver
// comentário lá pra detalhe completo de por que não é mais [id]/page.tsx.
function ContainerDetailPageInner() {
  const searchParams = useSearchParams();
  const id = searchParams.get("id") ?? "";
  return <ContainerDetailView containerId={id} />;
}

export default function ContainerDetailPage() {
  return (
    <Suspense>
      <ContainerDetailPageInner />
    </Suspense>
  );
}
