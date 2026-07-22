"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { AuthGate } from "@/components/auth/auth-gate";
import { ServerProvider } from "@/lib/server-context";

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 5_000,
            refetchInterval: 5_000,
            // Sem isso o polling para quando a aba perde foco — usuário troca de
            // aba enquanto o servidor sobe e volta pra ver status desatualizado.
            refetchIntervalInBackground: true,
          },
        },
      })
  );

  return (
    <QueryClientProvider client={queryClient}>
      <ServerProvider>
        <AuthGate>{children}</AuthGate>
      </ServerProvider>
    </QueryClientProvider>
  );
}
