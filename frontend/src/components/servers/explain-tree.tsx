"use client";

import { useState } from "react";
import type { PlanNode } from "@/lib/api";
import { ChevronDown, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";

function costColor(actualMs: number | undefined, totalMs: number | undefined) {
  if (actualMs == null || !totalMs) return "text-muted-foreground";
  const share = actualMs / totalMs;
  if (share >= 0.4) return "text-red-600";
  if (share >= 0.15) return "text-amber-600";
  return "text-muted-foreground";
}

function PlanNodeRow({ node, rootTotalMs }: { node: PlanNode; rootTotalMs?: number }) {
  const [open, setOpen] = useState(true);
  const children = node.Plans ?? [];
  const rowLabel = [node["Relation Name"], node["Index Name"], node["Alias"]]
    .filter(Boolean)
    .join(" · ");

  return (
    <div>
      <div className="flex items-start gap-1 py-1">
        {children.length > 0 ? (
          <button onClick={() => setOpen((v) => !v)} className="mt-0.5 shrink-0">
            {open ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
          </button>
        ) : (
          <span className="w-3.5 shrink-0" />
        )}
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-sm">
            <span className="font-medium">{node["Node Type"]}</span>
            {rowLabel && <span className="text-muted-foreground font-mono text-xs">{rowLabel}</span>}
            {node["Join Type"] && (
              <span className="text-muted-foreground text-xs">({node["Join Type"]})</span>
            )}
          </div>
          <div className="text-muted-foreground flex flex-wrap gap-x-3 text-xs">
            {node["Actual Total Time"] != null && (
              <span className={costColor(node["Actual Total Time"], rootTotalMs)}>
                {node["Actual Total Time"].toFixed(2)}ms
                {node["Actual Loops"] && node["Actual Loops"] > 1 ? ` × ${node["Actual Loops"]}` : ""}
              </span>
            )}
            {node["Actual Rows"] != null && node["Plan Rows"] != null && (
              <span
                className={
                  node["Actual Rows"] > node["Plan Rows"] * 5 || node["Actual Rows"] * 5 < node["Plan Rows"]
                    ? "text-amber-600"
                    : undefined
                }
              >
                linhas: {node["Actual Rows"]} (estimado {node["Plan Rows"]})
              </span>
            )}
            {node["Total Cost"] != null && <span>custo: {node["Total Cost"].toFixed(1)}</span>}
            {node["Filter"] && <span className="font-mono">filtro: {node["Filter"]}</span>}
            {node["Index Cond"] && <span className="font-mono">cond: {node["Index Cond"]}</span>}
          </div>
        </div>
      </div>
      {open && children.length > 0 && (
        <div className={cn("ml-4 border-l pl-3")}>
          {children.map((child, i) => (
            <PlanNodeRow key={i} node={child} rootTotalMs={rootTotalMs} />
          ))}
        </div>
      )}
    </div>
  );
}

export function ExplainTree({
  plan,
  planningTimeMs,
  executionTimeMs,
}: {
  plan: PlanNode;
  planningTimeMs?: number;
  executionTimeMs?: number;
}) {
  return (
    <div className="flex flex-col gap-2">
      {(planningTimeMs != null || executionTimeMs != null) && (
        <div className="text-muted-foreground flex gap-4 text-xs">
          {planningTimeMs != null && <span>Planejamento: {planningTimeMs.toFixed(2)}ms</span>}
          {executionTimeMs != null && <span>Execução: {executionTimeMs.toFixed(2)}ms</span>}
        </div>
      )}
      <PlanNodeRow node={plan} rootTotalMs={executionTimeMs ?? plan["Actual Total Time"]} />
    </div>
  );
}
