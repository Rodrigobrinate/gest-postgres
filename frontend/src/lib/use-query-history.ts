"use client";

import { useCallback, useState } from "react";

const MAX_HISTORY = 20;

function readHistory(key: string): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(key);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return []; // localStorage indisponível ou com lixo — segue sem histórico
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
          localStorage.setItem(key, JSON.stringify(next));
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
