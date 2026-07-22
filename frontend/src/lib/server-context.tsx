"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";

const STORAGE_KEY = "gestpg_selected_installation";

// currentServerId fica fora do React de propósito — lib/api.ts (função
// pura, sem hook) precisa ler o valor atual pra prefixar `/proxy/{id}` nas
// chamadas (ver MULTI_SERVER_MODE), e não dá pra chamar useContext() de
// dentro de uma função solta. ServerProvider é a ÚNICA coisa que escreve
// aqui, sempre em sincronia com o state React (useEffect abaixo).
let currentServerId: string | null = null;
export function getCurrentServerId(): string | null {
  return currentServerId;
}

type ServerContextValue = {
  selectedServerId: string | null;
  selectServer: (id: string | null) => void;
};

const ServerContext = createContext<ServerContextValue | null>(null);

// ServerProvider existe sempre (ver providers.tsx), mesmo fora do modo
// multi-instalação — custo zero: sem MULTI_SERVER_MODE, nada nunca chama
// selectServer, currentServerId fica null pra sempre, api.ts nunca prefixa
// nada (comportamento de hoje, intocado).
export function ServerProvider({ children }: { children: ReactNode }) {
  // Lazy initializer, não useEffect+setState — client components são
  // pré-renderizados em build time (sem `window`), daí a checagem: nesse
  // momento devolve null (mesmo estado que existiria antes da hidratação
  // de qualquer forma), no navegador de verdade lê o valor salvo direto.
  const [selectedServerId, setSelectedServerId] = useState<string | null>(() =>
    typeof window === "undefined" ? null : localStorage.getItem(STORAGE_KEY)
  );

  useEffect(() => {
    currentServerId = selectedServerId;
  }, [selectedServerId]);

  const selectServer = (id: string | null) => {
    setSelectedServerId(id);
    if (id) localStorage.setItem(STORAGE_KEY, id);
    else localStorage.removeItem(STORAGE_KEY);
  };

  return (
    <ServerContext.Provider value={{ selectedServerId, selectServer }}>
      {children}
    </ServerContext.Provider>
  );
}

export function useSelectedServer(): ServerContextValue {
  const ctx = useContext(ServerContext);
  if (!ctx) throw new Error("useSelectedServer precisa estar dentro de ServerProvider");
  return ctx;
}
