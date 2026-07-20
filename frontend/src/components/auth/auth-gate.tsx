"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

// AuthGate bloqueia a UI protegida até confirmar sessão válida via
// GET /api/v1/auth/me. É uma checagem client-side only — o cookie de sessão
// é do domínio/origem do BACKEND (NEXT_PUBLIC_API_URL), não do frontend, então
// o middleware.ts do Next (que roda no servidor do frontend) não teria como
// enxergar esse cookie mesmo sendo httpOnly. A API em si já exige sessão em
// toda rota (ver internal/api/middleware.go withAuth) — isso aqui é só pra
// não piscar UI protegida antes do redirect.
export function AuthGate({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const isLoginPage = pathname === "/login";

  const { isLoading, isError } = useQuery({
    queryKey: ["auth", "me"],
    queryFn: () => api.me(),
    enabled: !isLoginPage,
    retry: false,
    staleTime: 60_000,
    refetchInterval: false,
  });

  useEffect(() => {
    if (!isLoginPage && isError) {
      router.replace("/login");
    }
  }, [isLoginPage, isError, router]);

  if (isLoginPage) return <>{children}</>;
  if (isLoading || isError) return null;
  return <>{children}</>;
}
