"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import CodeMirror from "@uiw/react-codemirror";
import { ApiError, type FileEntry } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Download,
  File as FileIcon,
  Folder,
  Info,
  Pencil,
  Trash2,
  Upload,
} from "lucide-react";

export interface FileBrowserAdapter {
  list: (path: string) => Promise<FileEntry[]>;
  stat: (path: string) => Promise<FileEntry>;
  read: (path: string) => Promise<{ content: string }>;
  write: (path: string, content: string) => Promise<unknown>;
  upload: (path: string, file: File) => Promise<unknown>;
  remove: (path: string) => Promise<unknown>;
  downloadUrl: (path: string) => string;
}

function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(2)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

function formatMode(mode: number) {
  return "0" + (mode & 0o777).toString(8);
}

function joinPath(dir: string, name: string) {
  return dir === "/" ? `/${name}` : `${dir}/${name}`;
}

export function FileBrowser({
  adapter,
  queryKeyPrefix,
  readOnly,
}: {
  adapter: FileBrowserAdapter;
  queryKeyPrefix: string;
  /** Esconde editar/upload/excluir — usado antes do step-up no gerenciador
   * de arquivos do host (ver host-files-tab.tsx). Listar/ler/baixar continuam
   * liberados, só escrita/exclusão exigem elevação. */
  readOnly?: boolean;
}) {
  const [currentPath, setCurrentPath] = useState("/");
  const [editing, setEditing] = useState<FileEntry | null>(null);
  const [properties, setProperties] = useState<FileEntry | null>(null);

  const queryClient = useQueryClient();
  const listKey = [queryKeyPrefix, "files", currentPath];

  const { data: entries, isLoading } = useQuery({
    queryKey: listKey,
    queryFn: () => adapter.list(currentPath),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: [queryKeyPrefix, "files"] });

  const remove = useMutation({
    mutationFn: (path: string) => adapter.remove(path),
    onSuccess: () => {
      toast.success("Removido");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao remover"),
  });

  const upload = useMutation({
    mutationFn: (file: File) => adapter.upload(currentPath, file),
    onSuccess: () => {
      toast.success("Upload concluído");
      invalidate();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha no upload"),
  });

  const segments = currentPath === "/" ? [] : currentPath.split("/").filter(Boolean);

  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-1 text-sm">
          <button className="hover:underline" onClick={() => setCurrentPath("/")}>
            /
          </button>
          {segments.map((seg, i) => {
            const path = "/" + segments.slice(0, i + 1).join("/");
            return (
              <span key={path} className="flex items-center gap-1">
                <span className="text-muted-foreground">/</span>
                <button className="hover:underline" onClick={() => setCurrentPath(path)}>
                  {seg}
                </button>
              </span>
            );
          })}
        </div>
        {!readOnly && (
        <label>
          <input
            type="file"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) upload.mutate(file);
              e.target.value = "";
            }}
          />
          <Button size="sm" variant="outline" disabled={upload.isPending} render={<span />}>
            <Upload className="size-4" />
            {upload.isPending ? "Enviando..." : "Upload"}
          </Button>
        </label>
        )}
      </div>

      <div className="overflow-x-auto rounded-md border">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-muted-foreground border-b text-xs">
              <th className="px-3 py-2 text-left font-normal">Nome</th>
              <th className="px-3 py-2 text-left font-normal">Tamanho</th>
              <th className="px-3 py-2 text-left font-normal">Modificado</th>
              <th className="px-3 py-2 text-right font-normal">Ações</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={4} className="text-muted-foreground px-3 py-6 text-center">
                  Carregando...
                </td>
              </tr>
            ) : !entries || entries.length === 0 ? (
              <tr>
                <td colSpan={4} className="text-muted-foreground px-3 py-6 text-center">
                  Pasta vazia.
                </td>
              </tr>
            ) : (
              entries.map((entry) => {
                const fullPath = entry.path || joinPath(currentPath, entry.name);
                return (
                  <tr key={entry.name} className="group border-b last:border-0">
                    <td className="px-3 py-2">
                      {entry.is_dir ? (
                        <button
                          className="flex items-center gap-1.5 hover:underline"
                          onClick={() => setCurrentPath(fullPath)}
                        >
                          <Folder className="text-muted-foreground size-4" />
                          {entry.name}
                        </button>
                      ) : (
                        <span className="flex items-center gap-1.5">
                          <FileIcon className="text-muted-foreground size-4" />
                          {entry.name}
                        </span>
                      )}
                    </td>
                    <td className="text-muted-foreground px-3 py-2 text-xs">
                      {entry.is_dir ? "—" : formatBytes(entry.size)}
                    </td>
                    <td className="text-muted-foreground px-3 py-2 text-xs">
                      {entry.mod_time ? new Date(entry.mod_time).toLocaleString("pt-BR") : "—"}
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex justify-end gap-1 opacity-0 group-hover:opacity-100">
                        {!readOnly && !entry.is_dir && (
                          <Button
                            size="icon-xs"
                            variant="ghost"
                            title="Editar"
                            onClick={() => setEditing(entry)}
                          >
                            <Pencil className="size-3.5" />
                          </Button>
                        )}
                        <Button
                          size="icon-xs"
                          variant="ghost"
                          title="Propriedades"
                          onClick={() => setProperties(entry)}
                        >
                          <Info className="size-3.5" />
                        </Button>
                        <Button
                          size="icon-xs"
                          variant="ghost"
                          title={entry.is_dir ? "Baixar (.tar.gz)" : "Baixar"}
                          render={<a href={adapter.downloadUrl(fullPath)} />}
                        >
                          <Download className="size-3.5" />
                        </Button>
                        {!readOnly && (
                          <Button
                            size="icon-xs"
                            variant="ghost"
                            className="text-red-600"
                            title="Excluir"
                            disabled={remove.isPending}
                            onClick={() => {
                              if (confirm(`Excluir "${entry.name}"? Sem volta.`)) {
                                remove.mutate(fullPath);
                              }
                            }}
                          >
                            <Trash2 className="size-3.5" />
                          </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {editing && (
        <EditFileDialog
          adapter={adapter}
          entry={editing}
          path={editing.path || joinPath(currentPath, editing.name)}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            invalidate();
          }}
        />
      )}

      {properties && (
        <Dialog open onOpenChange={(v) => !v && setProperties(null)}>
          <DialogContent className="sm:max-w-sm">
            <DialogHeader>
              <DialogTitle className="font-mono text-sm">{properties.name}</DialogTitle>
            </DialogHeader>
            <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-sm">
              <div className="text-muted-foreground">Tipo</div>
              <div>{properties.is_dir ? "Pasta" : "Arquivo"}</div>
              <div className="text-muted-foreground">Tamanho</div>
              <div>{formatBytes(properties.size)}</div>
              <div className="text-muted-foreground">Permissão</div>
              <div className="font-mono">{formatMode(properties.mode)}</div>
              <div className="text-muted-foreground">Modificado</div>
              <div>
                {properties.mod_time ? new Date(properties.mod_time).toLocaleString("pt-BR") : "—"}
              </div>
            </div>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

function EditFileDialog({
  adapter,
  entry,
  path,
  onClose,
  onSaved,
}: {
  adapter: FileBrowserAdapter;
  entry: FileEntry;
  path: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { data, isLoading } = useQuery({
    queryKey: ["file-content", path],
    queryFn: () => adapter.read(path),
  });
  const [content, setContent] = useState<string | null>(null);

  const save = useMutation({
    mutationFn: () => adapter.write(path, content ?? ""),
    onSuccess: () => {
      toast.success("Salvo");
      onSaved();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Falha ao salvar"),
  });

  const value = content ?? data?.content ?? "";

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle className="font-mono text-sm">{entry.name}</DialogTitle>
        </DialogHeader>
        {isLoading ? (
          <p className="text-muted-foreground text-sm">Carregando...</p>
        ) : (
          <div className="overflow-hidden rounded-md border">
            <CodeMirror
              value={value}
              height="420px"
              onChange={(v) => setContent(v)}
              basicSetup={{ lineNumbers: true }}
              className="text-sm"
            />
          </div>
        )}
        <DialogFooter>
          <Button disabled={isLoading || save.isPending} onClick={() => save.mutate()}>
            {save.isPending ? "Salvando..." : "Salvar"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
