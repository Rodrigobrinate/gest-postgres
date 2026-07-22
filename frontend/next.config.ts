import type { NextConfig } from "next";

// output: "export" — o frontend deixa de rodar como servidor Node
// (standalone/`next start`) e vira arquivos estáticos, hospedáveis no
// Cloudflare Pages. Investigação confirmou zero bloqueio arquitetural: toda
// busca de dado já é client-side (TanStack Query + fetch direto do
// navegador via lib/api.ts), zero Server Actions/Route Handlers/
// middleware.ts. CSP/X-Frame-Options/X-Content-Type-Options saem daqui
// (headers() não existe em export estático) e viram public/_headers, que o
// Cloudflare Pages aplica automaticamente — cópia do mesmo conteúdo, só o
// formato muda.
const nextConfig: NextConfig = {
  output: "export",
};

export default nextConfig;
