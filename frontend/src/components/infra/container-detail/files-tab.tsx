"use client";

import { api, type MountInfo } from "@/lib/api";
import { FileBrowser, type FileBrowserAdapter } from "@/components/infra/file-browser";

export function FilesTab({ containerId, mounts }: { containerId: string; mounts: MountInfo[] }) {
  const adapter: FileBrowserAdapter = {
    list: (path) => api.listContainerFiles(containerId, path),
    stat: (path) => api.statContainerFile(containerId, path),
    read: (path) => api.readContainerFile(containerId, path),
    write: (path, content) => api.writeContainerFile(containerId, path, content),
    upload: (path, file) => api.uploadContainerFile(containerId, path, file),
    remove: (path) => api.deleteContainerFile(containerId, path),
    downloadUrl: (path) => api.containerFileDownloadUrl(containerId, path),
  };

  return (
    <div className="grid gap-3">
      {mounts.some((m) => m.type === "volume") && (
        <p className="text-muted-foreground text-xs">
          Navega pelo filesystem inteiro do container, incluindo dentro dos volumes montados
          (marcados na aba Volumes).
        </p>
      )}
      <FileBrowser adapter={adapter} queryKeyPrefix={`container-${containerId}`} />
    </div>
  );
}
