"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import mermaid from "mermaid";
import { api, type ERD } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { RefreshCw, ZoomIn, ZoomOut, RotateCcw } from "lucide-react";

mermaid.initialize({
  startOnLoad: false,
  theme: "neutral",
  // 'strict' (default do mermaid) sanitiza o SVG de saída via DOMPurify
  // internamente antes de devolver — setado explícito aqui porque esse é o
  // único lugar do frontend que injeta HTML via innerHTML em vez de deixar
  // o React renderizar (necessário: mermaid.render devolve uma string SVG,
  // não JSX). Sem isso seria a única exceção não documentada à regra do
  // projeto de nunca usar innerHTML/dangerouslySetInnerHTML.
  securityLevel: "strict",
});

// sanitizeId vira identificador válido pro parser do mermaid — nome de
// tabela/coluna do Postgres é bem mais permissivo que isso (aceita quase
// qualquer caractere se vier entre aspas), então nomes fora do comum
// (espaço, acento, etc) precisam virar algo previsível aqui. É só pro
// diagrama; o nome real continua correto em qualquer outro lugar da UI
// (aba Tabelas, Editor SQL).
function sanitizeId(s: string) {
  return s.replace(/[^a-zA-Z0-9_]/g, "_");
}

function entityId(schema: string, table: string) {
  return schema === "public" ? sanitizeId(table) : sanitizeId(`${schema}_${table}`);
}

function entityLabel(schema: string, table: string) {
  return schema === "public" ? table : `${schema}.${table}`;
}

function buildMermaidSource(erd: ERD): string {
  if (erd.tables.length === 0) return "";

  const fkColumns = new Set(
    erd.relationships.map((r) => `${r.from_schema}.${r.from_table}.${r.from_column}`)
  );

  const lines: string[] = ["erDiagram"];

  for (const rel of erd.relationships) {
    const parent = entityId(rel.to_schema, rel.to_table);
    const child = entityId(rel.from_schema, rel.from_table);
    // Assume 1-pra-N (pai ||--o{ filho) — o caso overwhelmingly comum de FK
    // apontando pra uma PK/UNIQUE. Não introspecciona se a coluna FK em si
    // também é UNIQUE (viraria 1-pra-1, ||--||) — refinamento de backlog,
    // não muda a topologia do diagrama, só o símbolo da ponta.
    lines.push(`    ${parent} ||--o{ ${child} : "${sanitizeId(rel.constraint_name)}"`);
  }

  for (const t of erd.tables) {
    const id = entityId(t.schema, t.name);
    const label = entityLabel(t.schema, t.name);
    lines.push(id === label ? `    ${id} {` : `    ${id}["${label}"] {`);
    for (const c of t.columns) {
      const type = sanitizeId(c.type) || "unknown";
      const keys: string[] = [];
      if (c.primary_key) keys.push("PK");
      if (fkColumns.has(`${t.schema}.${t.name}.${c.name}`)) keys.push("FK");
      const keySuffix = keys.length ? ` ${keys.join(",")}` : "";
      lines.push(`        ${type} ${sanitizeId(c.name)}${keySuffix}`);
    }
    lines.push("    }");
  }

  return lines.join("\n");
}

let renderCounter = 0;

export function ErdTab({ serverId, database }: { serverId: string; database: string }) {
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ["servers", serverId, "erd", database],
    queryFn: () => api.getERD(serverId, database),
    enabled: !!database,
  });

  const containerRef = useRef<HTMLDivElement>(null);
  const [renderError, setRenderError] = useState<string | null>(null);
  const [zoom, setZoom] = useState(1);

  const source = useMemo(() => (data ? buildMermaidSource(data) : ""), [data]);

  useEffect(() => {
    if (!source || !containerRef.current) return;
    let cancelled = false;
    const id = `erd-${++renderCounter}`;
    mermaid
      .render(id, source)
      .then(({ svg }) => {
        if (cancelled || !containerRef.current) return;
        containerRef.current.innerHTML = svg;
        setRenderError(null);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setRenderError(err instanceof Error ? err.message : "Falha renderizando o diagrama");
      });
    return () => {
      cancelled = true;
    };
  }, [source]);

  const tableCount = data?.tables.length ?? 0;
  const relCount = data?.relationships.length ?? 0;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-muted-foreground text-sm">
          {isLoading
            ? "Carregando..."
            : `${tableCount} tabela(s) · ${relCount} relacionamento(s) de chave estrangeira`}
        </p>
        <div className="flex items-center gap-1">
          <Button size="icon" variant="outline" onClick={() => setZoom((z) => Math.max(0.3, z - 0.15))}>
            <ZoomOut className="size-4" />
          </Button>
          <Button size="icon" variant="outline" onClick={() => setZoom(1)}>
            <RotateCcw className="size-4" />
          </Button>
          <Button size="icon" variant="outline" onClick={() => setZoom((z) => Math.min(2.5, z + 0.15))}>
            <ZoomIn className="size-4" />
          </Button>
          <Button size="sm" variant="outline" disabled={isFetching} onClick={() => refetch()}>
            <RefreshCw className={isFetching ? "size-3.5 animate-spin" : "size-3.5"} />
            Atualizar
          </Button>
        </div>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : tableCount === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">
              Nenhuma tabela em &ldquo;{database}&rdquo; ainda.
            </p>
          ) : renderError ? (
            <p className="p-6 text-sm text-red-600">Falha renderizando o diagrama: {renderError}</p>
          ) : (
            <div className="overflow-auto p-4" style={{ maxHeight: "70vh" }}>
              <div
                ref={containerRef}
                style={{ transform: `scale(${zoom})`, transformOrigin: "top left" }}
              />
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
