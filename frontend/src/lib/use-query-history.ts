"use client";

import { useCallback, useState } from "react";

const MAX_HISTORY = 20;

function readHistory(key: string): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = sessionStorage.getItem(key);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return []; // sessionStorage indisponível ou com lixo — segue sem histórico
  }
}

// clearAllQueryHistory apaga o histórico de SQL de TODOS os servidores —
// chamado no logout. sessionStorage já não sobrevive a fechar o navegador,
// mas nada limpa sozinho ao trocar de sessão/usuário na mesma aba sem isso
// (histórico de SQL rotineiramente tem senha em CREATE ROLE ... PASSWORD, PII).
export function clearAllQueryHistory() {
  if (typeof window === "undefined") return;
  for (let i = sessionStorage.length - 1; i >= 0; i--) {
    const k = sessionStorage.key(i);
    if (k?.startsWith("sql-history:")) sessionStorage.removeItem(k);
  }
}

export function useQueryHistory(key: string) {
  const [history, setHistory] = useState<string[]>(() => readHistory(key));

  const push = useCallback(
    (sql: string) => {
      const trimmed = sql.trim();
      if (!trimmed) return;
      setHistory((prev) => {
        const next = [trimmed, ...prev.filter((q) => q !== trimmed)].slice(0, MAX_HISTORY);
        try {
          sessionStorage.setItem(key, JSON.stringify(next));
        } catch {
          // ignora falha ao persistir (storage cheio/desabilitado)
        }
        return next;
      });
    },
    [key]
  );

  return { history, push };
}
