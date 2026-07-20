import { ContainerDetailView } from "@/components/infra/container-detail/container-detail-view";

export default async function ContainerDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <ContainerDetailView containerId={id} />;
}
