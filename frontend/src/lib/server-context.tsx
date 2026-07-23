"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";

const STORAGE_KEY = "gestpg_selected_installation";
const URL_PARAM = "installation";

export interface SelectedServer {
  id: string;
  name: string;
}

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
  selectedServer: SelectedServer | null;
  selectServer: (server: SelectedServer | null) => void;
};

const ServerContext = createContext<ServerContextValue | null>(null);

function readStoredServer(): SelectedServer | null {
  if (typeof window === "undefined") return null;
  // URL manda (link direto/compartilhado/recarregar a página) — só o id
  // vem por ali, nome fica em branco até a próxima seleção via card (não
  // vale a pena buscar a lista inteira só pra achar o nome de um id já
  // conhecido). localStorage é o fallback pra navegação normal dentro do
  // app (nome já foi visto quando selecionou pela overview).
  const urlId = new URLSearchParams(window.location.search).get(URL_PARAM);
  if (urlId) {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      try {
        const parsed = JSON.parse(stored) as SelectedServer;
        if (parsed.id === urlId) return parsed;
      } catch {
        // localStorage corrompido/formato antigo, ignora
      }
    }
    return { id: urlId, name: "" };
  }
  const stored = localStorage.getItem(STORAGE_KEY);
  if (!stored) return null;
  try {
    return JSON.parse(stored) as SelectedServer;
  } catch {
    return null;
  }
}

function syncUrl(server: SelectedServer | null) {
  const url = new URL(window.location.href);
  if (server) url.searchParams.set(URL_PARAM, server.id);
  else url.searchParams.delete(URL_PARAM);
  window.history.replaceState(null, "", url.toString());
}

// ServerProvider existe sempre (ver providers.tsx), mesmo fora do modo
// multi-instalação — custo zero: sem MULTI_SERVER_MODE, nada nunca chama
// selectServer, currentServerId fica null pra sempre, api.ts nunca prefixa
// nada (comportamento de hoje, intocado).
export function ServerProvider({ children }: { children: ReactNode }) {
  // Lazy initializer, não useEffect+setState — client components são
  // pré-renderizados em build time (sem `window`), daí a checagem: nesse
  // momento devolve null (mesmo estado que existiria antes da hidratação
  // de qualquer forma), no navegador de verdade lê o valor salvo/da URL.
  const [selectedServer, setSelectedServer] = useState<SelectedServer | null>(readStoredServer);
  const queryClient = useQueryClient();

  useEffect(() => {
    currentServerId = selectedServer?.id ?? null;
  }, [selectedServer]);

  const selectServer = (server: SelectedServer | null) => {
    // Achado ao vivo: trocar de instalação sem limpar o cache do TanStack
    // Query mostrava o dado da instalação ANTERIOR até o fetch novo
    // sobrescrever — toda query nesse app usa a mesma chave (ex: ["servers"])
    // pra qualquer instalação, já que o prefixo /proxy/{id} é decidido em
    // runtime dentro de apiPath(), não faz parte da query key. Sem uma
    // chave por instalação (mudaria centenas de useQuery espalhados pelo
    // app inteiro), a saída mais simples é limpar tudo na troca — cada
    // componente busca de novo do zero, mostra "carregando" de verdade em
    // vez de piscar dado errado.
    currentServerId = server?.id ?? null; // síncrono, antes do clear — apiPath já lê o valor certo na hora do refetch
    setSelectedServer(server);
    if (server) localStorage.setItem(STORAGE_KEY, JSON.stringify(server));
    else localStorage.removeItem(STORAGE_KEY);
    syncUrl(server);
    queryClient.clear();
  };

  return (
    <ServerContext.Provider value={{ selectedServer, selectServer }}>{children}</ServerContext.Provider>
  );
}

export function useSelectedServer(): ServerContextValue {
  const ctx = useContext(ServerContext);
  if (!ctx) throw new Error("useSelectedServer precisa estar dentro de ServerProvider");
  return ctx;
}
