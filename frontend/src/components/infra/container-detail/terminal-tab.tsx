"use client";

import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Button } from "@/components/ui/button";
import { TerminalSquare } from "lucide-react";
import { API_URL, apiPath } from "@/lib/api";

// Terminal interativo dentro do container, via WebSocket
// (backend: internal/api/terminal.go, usa EXEC do docker-socket-proxy). Não
// conecta sozinho ao montar — só quando o usuário clica "Abrir terminal",
// pra não abrir um shell num container toda vez que a página de detalhe é
// visitada e essa aba só está pré-renderizada, escondida.
export function TerminalTab({ containerId }: { containerId: string }) {
  const [connected, setConnected] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!connected || !containerRef.current) return;

    const term = new Terminal({
      convertEol: true,
      fontSize: 13,
      theme: { background: "#0a0a0a" },
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(containerRef.current);
    fitAddon.fit();

    const wsUrl = `${API_URL.replace(/^http/, "ws")}${apiPath(`/api/v1/infra/containers/${containerId}/exec`)}`;
    const ws = new WebSocket(wsUrl);
    ws.binaryType = "arraybuffer";

    ws.onmessage = (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data));
      } else {
        term.write(ev.data as string);
      }
    };
    ws.onopen = () => {
      ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    };
    ws.onclose = () => {
      term.write("\r\n\x1b[31m[sessão encerrada]\x1b[0m\r\n");
    };

    const dataDisposable = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "stdin", data }));
      }
    });

    const handleResize = () => {
      fitAddon.fit();
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    };
    const resizeObserver = new ResizeObserver(handleResize);
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      dataDisposable.dispose();
      ws.close();
      term.dispose();
    };
  }, [connected, containerId]);

  if (!connected) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-10">
        <TerminalSquare className="text-muted-foreground size-8" />
        <p className="text-muted-foreground max-w-sm text-center text-sm">
          Abre um shell dentro do container — qualquer comando roda com os
          mesmos privilégios do processo principal dele.
        </p>
        <Button onClick={() => setConnected(true)}>Abrir terminal</Button>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="h-[480px] overflow-hidden rounded-lg border bg-[#0a0a0a] p-2"
    />
  );
}
