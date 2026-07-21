"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { clearAllQueryHistory } from "@/lib/use-query-history";
import { CurrentUserProvider } from "./current-user";

// AuthGate bloqueia a UI protegida até confirmar sessão válida via
// GET /api/v1/auth/me — e disponibiliza usuário/papel atual pro resto da
// árvore via contexto (ver current-user.tsx), pra esconder ação
// admin-only (gerenciar usuário, escrever no file manager do host etc) de
// quem logou como viewer. É uma checagem client-side only — o cookie de
// sessão é do domínio/origem do BACKEND (NEXT_PUBLIC_API_URL), não do
// frontend, então o middleware.ts do Next (que roda no servidor do
// frontend) não teria como enxergar esse cookie mesmo sendo httpOnly. A
// API em si já exige sessão (e papel certo) em toda rota — isso aqui é só
// pra não piscar UI protegida antes do redirect.
export function AuthGate({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const isLoginPage = pathname === "/login";

  const { data, isLoading, isError } = useQuery({
    queryKey: ["auth", "me"],
    queryFn: () => api.me(),
    enabled: !isLoginPage,
    retry: false,
    staleTime: 60_000,
    refetchInterval: false,
  });

  useEffect(() => {
    if (!isLoginPage && isError) {
      // Mesma limpeza do botão de logout — sessão inválida/expirada
      // detectada aqui é um logout automático, histórico de SQL não pode
      // sobreviver na aba pro próximo usuário que logar (achado de
      // auditoria: só o botão de logout limpava).
      clearAllQueryHistory();
      router.replace("/login");
    }
  }, [isLoginPage, isError, router]);

  if (isLoginPage) return <>{children}</>;
  if (isLoading || isError || !data) return null;

  return (
    <CurrentUserProvider value={{ username: data.username, role: data.role }}>
      {children}
    </CurrentUserProvider>
  );
}
