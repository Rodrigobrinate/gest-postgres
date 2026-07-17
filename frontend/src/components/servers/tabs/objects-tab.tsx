"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, ApiError, COLUMN_TYPES } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { Plus, RefreshCw, Trash2 } from "lucide-react";

type Kind = "views" | "matviews" | "sequences" | "types";

const KINDS: { value: Kind; label: string }[] = [
  { value: "views", label: "Views" },
  { value: "matviews", label: "Materialized Views" },
  { value: "sequences", label: "Sequences" },
  { value: "types", label: "Types / Domains" },
];

export function ObjectsTab({ serverId, database }: { serverId: string; database: string }) {
  const [kind, setKind] = useState<Kind>("views");

  return (
    <div className="flex flex-col gap-4">
      <div className="flex gap-1">
        {KINDS.map((k) => (
          <button
            key={k.value}
            onClick={() => setKind(k.value)}
            className={cn(
              "rounded-md px-3 py-1.5 text-sm",
              kind === k.value ? "bg-primary text-primary-foreground" : "hover:bg-accent"
            )}
          >
            {k.label}
          </button>
        ))}
      </div>

      {kind === "views" && <ViewsPanel serverId={serverId} database={database} />}
      {kind === "matviews" && <MatViewsPanel serverId={serverId} database={database} />}
      {kind === "sequences" && <SequencesPanel serverId={serverId} database={database} />}
      {kind === "types" && <TypesPanel serverId={serverId} database={database} />}
    </div>
  );
}

// ---------- Views ----------

function ViewsPanel({ serverId, database }: { serverId: string; database: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["servers", serverId, "views", database],
    queryFn: () => api.listViews(serverId, database),
    enabled: !!database,
  });
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "views"] });

  const [open, setOpen] = useState(false);
  const [schema, setSchema] = useState("public");
  const [name, setName] = useState("");
  const [query, setQuery] = useState("");

  const create = useMutation({
    mutationFn: () => api.createView(serverId, database, schema, name, query),
    onSuccess: () => {
      toast.success(`View "${name}" criada`);
      setOpen(false);
      setName("");
      setQuery("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar view"),
  });

  const remove = useMutation({
    mutationFn: (v: { schema: string; name: string }) => api.dropView(serverId, database, v.schema, v.name),
    onSuccess: (_d, v) => {
      toast.success(`${v.name} excluída`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir view"),
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" variant="outline" />}>
            <Plus className="size-4" />
            Nova view
          </DialogTrigger>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle>Criar view</DialogTitle>
            </DialogHeader>
            <div className="grid gap-3 py-2">
              <div className="grid grid-cols-[1fr_2fr] gap-3">
                <div className="grid gap-1.5">
                  <Label>Schema</Label>
                  <Input value={schema} onChange={(e) => setSchema(e.target.value)} />
                </div>
                <div className="grid gap-1.5">
                  <Label>Nome</Label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </div>
              </div>
              <div className="grid gap-1.5">
                <Label>Query (SELECT que define a view)</Label>
                <Textarea
                  rows={5}
                  className="font-mono text-xs"
                  placeholder="SELECT * FROM ..."
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                disabled={create.isPending || !name.trim() || !query.trim()}
                onClick={() => create.mutate()}
              >
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !data || data.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhuma view em &ldquo;{database}&rdquo;.</p>
          ) : (
            <ul className="divide-y">
              {data.map((v) => (
                <li key={`${v.schema}.${v.name}`} className="flex items-start justify-between gap-3 p-3">
                  <div className="min-w-0">
                    <span className="font-mono text-sm font-medium">
                      {v.schema}.{v.name}
                    </span>
                    <p className="text-muted-foreground truncate font-mono text-xs">{v.definition}</p>
                  </div>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="shrink-0 text-red-600"
                    onClick={() => remove.mutate({ schema: v.schema, name: v.name })}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// ---------- Materialized Views ----------

function MatViewsPanel({ serverId, database }: { serverId: string; database: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["servers", serverId, "matviews", database],
    queryFn: () => api.listMaterializedViews(serverId, database),
    enabled: !!database,
  });
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "matviews"] });

  const [open, setOpen] = useState(false);
  const [schema, setSchema] = useState("public");
  const [name, setName] = useState("");
  const [query, setQuery] = useState("");

  const create = useMutation({
    mutationFn: () => api.createMaterializedView(serverId, database, schema, name, query),
    onSuccess: () => {
      toast.success(`Materialized view "${name}" criada`);
      setOpen(false);
      setName("");
      setQuery("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar"),
  });

  const refresh = useMutation({
    mutationFn: (v: { schema: string; name: string }) =>
      api.refreshMaterializedView(serverId, database, v.schema, v.name),
    onSuccess: (_d, v) => {
      toast.success(`${v.name} atualizada`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao atualizar"),
  });

  const remove = useMutation({
    mutationFn: (v: { schema: string; name: string }) =>
      api.dropMaterializedView(serverId, database, v.schema, v.name),
    onSuccess: (_d, v) => {
      toast.success(`${v.name} excluída`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir"),
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" variant="outline" />}>
            <Plus className="size-4" />
            Nova materialized view
          </DialogTrigger>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle>Criar materialized view</DialogTitle>
            </DialogHeader>
            <div className="grid gap-3 py-2">
              <div className="grid grid-cols-[1fr_2fr] gap-3">
                <div className="grid gap-1.5">
                  <Label>Schema</Label>
                  <Input value={schema} onChange={(e) => setSchema(e.target.value)} />
                </div>
                <div className="grid gap-1.5">
                  <Label>Nome</Label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </div>
              </div>
              <div className="grid gap-1.5">
                <Label>Query</Label>
                <Textarea
                  rows={5}
                  className="font-mono text-xs"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                disabled={create.isPending || !name.trim() || !query.trim()}
                onClick={() => create.mutate()}
              >
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !data || data.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhuma materialized view.</p>
          ) : (
            <ul className="divide-y">
              {data.map((v) => (
                <li key={`${v.schema}.${v.name}`} className="flex items-start justify-between gap-3 p-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-medium">
                        {v.schema}.{v.name}
                      </span>
                      <Badge variant="outline">{(v.size_bytes / 1024).toFixed(0)} KB</Badge>
                      {!v.populated && <Badge variant="outline">não populada</Badge>}
                    </div>
                    <p className="text-muted-foreground truncate font-mono text-xs">{v.definition}</p>
                  </div>
                  <div className="flex shrink-0 gap-1">
                    <Button
                      size="icon"
                      variant="ghost"
                      title="Atualizar (REFRESH)"
                      onClick={() => refresh.mutate({ schema: v.schema, name: v.name })}
                    >
                      <RefreshCw className="size-4" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="text-red-600"
                      onClick={() => remove.mutate({ schema: v.schema, name: v.name })}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// ---------- Sequences ----------

function SequencesPanel({ serverId, database }: { serverId: string; database: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["servers", serverId, "sequences", database],
    queryFn: () => api.listSequences(serverId, database),
    enabled: !!database,
  });
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "sequences"] });

  const [open, setOpen] = useState(false);
  const [schema, setSchema] = useState("public");
  const [name, setName] = useState("");
  const [increment, setIncrement] = useState(1);
  const [startWith, setStartWith] = useState(1);
  const [cycle, setCycle] = useState(false);

  const create = useMutation({
    mutationFn: () =>
      api.createSequence(serverId, database, { schema, name, increment, start_with: startWith, cycle }),
    onSuccess: () => {
      toast.success(`Sequence "${name}" criada`);
      setOpen(false);
      setName("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar sequence"),
  });

  const remove = useMutation({
    mutationFn: (v: { schema: string; name: string }) => api.dropSequence(serverId, database, v.schema, v.name),
    onSuccess: (_d, v) => {
      toast.success(`${v.name} excluída`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir"),
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" variant="outline" />}>
            <Plus className="size-4" />
            Nova sequence
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Criar sequence</DialogTitle>
            </DialogHeader>
            <div className="grid gap-3 py-2">
              <div className="grid grid-cols-[1fr_2fr] gap-3">
                <div className="grid gap-1.5">
                  <Label>Schema</Label>
                  <Input value={schema} onChange={(e) => setSchema(e.target.value)} />
                </div>
                <div className="grid gap-1.5">
                  <Label>Nome</Label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-1.5">
                  <Label>Incremento</Label>
                  <Input type="number" value={increment} onChange={(e) => setIncrement(Number(e.target.value))} />
                </div>
                <div className="grid gap-1.5">
                  <Label>Começa em</Label>
                  <Input type="number" value={startWith} onChange={(e) => setStartWith(Number(e.target.value))} />
                </div>
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={cycle} onChange={(e) => setCycle(e.target.checked)} />
                Cicla ao chegar no máximo
              </label>
            </div>
            <DialogFooter>
              <Button disabled={create.isPending || !name.trim()} onClick={() => create.mutate()}>
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !data || data.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhuma sequence.</p>
          ) : (
            <ul className="divide-y">
              {data.map((sq) => (
                <li key={`${sq.schema}.${sq.name}`} className="flex items-center justify-between gap-3 p-3">
                  <div>
                    <span className="font-mono text-sm font-medium">
                      {sq.schema}.{sq.name}
                    </span>
                    <p className="text-muted-foreground text-xs">
                      valor atual: {sq.last_value ?? "—"} · incremento: {sq.increment} · cache:{" "}
                      {sq.cache_size}
                      {sq.cycle && " · cicla"}
                    </p>
                  </div>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="text-red-600"
                    onClick={() => remove.mutate({ schema: sq.schema, name: sq.name })}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// ---------- Types / Domains ----------

function TypesPanel({ serverId, database }: { serverId: string; database: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["servers", serverId, "types", database],
    queryFn: () => api.listTypes(serverId, database),
    enabled: !!database,
  });
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["servers", serverId, "types"] });

  const [open, setOpen] = useState(false);
  const [typeKind, setTypeKind] = useState<"enum" | "domain">("enum");
  const [schema, setSchema] = useState("public");
  const [name, setName] = useState("");
  const [enumValues, setEnumValues] = useState("");
  const [baseType, setBaseType] = useState("text");
  const [checkExpr, setCheckExpr] = useState("");

  const create = useMutation({
    mutationFn: () =>
      typeKind === "enum"
        ? api.createEnumType(
            serverId,
            database,
            schema,
            name,
            enumValues.split(",").map((v) => v.trim()).filter(Boolean)
          )
        : api.createDomain(serverId, database, schema, name, baseType, checkExpr),
    onSuccess: () => {
      toast.success(`Type "${name}" criado`);
      setOpen(false);
      setName("");
      setEnumValues("");
      setCheckExpr("");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao criar type"),
  });

  const remove = useMutation({
    mutationFn: (v: { schema: string; name: string }) => api.dropType(serverId, database, v.schema, v.name),
    onSuccess: (_d, v) => {
      toast.success(`${v.name} excluído`);
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao excluir"),
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button size="sm" variant="outline" />}>
            <Plus className="size-4" />
            Novo type
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Criar type / domain</DialogTitle>
            </DialogHeader>
            <div className="grid gap-3 py-2">
              <div className="grid gap-1.5">
                <Label>Tipo</Label>
                <Select value={typeKind} onValueChange={(v) => v && setTypeKind(v as typeof typeKind)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="enum">Enum (lista fixa de valores)</SelectItem>
                    <SelectItem value="domain">Domain (tipo base + regra)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="grid grid-cols-[1fr_2fr] gap-3">
                <div className="grid gap-1.5">
                  <Label>Schema</Label>
                  <Input value={schema} onChange={(e) => setSchema(e.target.value)} />
                </div>
                <div className="grid gap-1.5">
                  <Label>Nome</Label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </div>
              </div>

              {typeKind === "enum" ? (
                <div className="grid gap-1.5">
                  <Label>Valores (separados por vírgula)</Label>
                  <Input
                    placeholder="pendente, pago, enviado"
                    value={enumValues}
                    onChange={(e) => setEnumValues(e.target.value)}
                  />
                </div>
              ) : (
                <>
                  <div className="grid gap-1.5">
                    <Label>Tipo base</Label>
                    <Select value={baseType} onValueChange={(v) => v && setBaseType(v)}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {COLUMN_TYPES.map((t) => (
                          <SelectItem key={t} value={t}>
                            {t}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="grid gap-1.5">
                    <Label>CHECK (opcional)</Label>
                    <Input
                      placeholder="VALUE ~ '^[^@]+@[^@]+$'"
                      value={checkExpr}
                      onChange={(e) => setCheckExpr(e.target.value)}
                    />
                  </div>
                </>
              )}
            </div>
            <DialogFooter>
              <Button disabled={create.isPending || !name.trim()} onClick={() => create.mutate()}>
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="text-muted-foreground p-6 text-sm">Carregando...</p>
          ) : !data || data.length === 0 ? (
            <p className="text-muted-foreground p-6 text-sm">Nenhum type/domain.</p>
          ) : (
            <ul className="divide-y">
              {data.map((t) => (
                <li key={`${t.schema}.${t.name}`} className="flex items-center justify-between gap-3 p-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-medium">
                        {t.schema}.{t.name}
                      </span>
                      <Badge variant="outline">{t.kind}</Badge>
                    </div>
                    <p className="text-muted-foreground truncate text-xs">{t.detail}</p>
                  </div>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="shrink-0 text-red-600"
                    onClick={() => remove.mutate({ schema: t.schema, name: t.name })}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
