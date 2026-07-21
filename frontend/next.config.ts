import type { NextConfig } from "next";

// connect-src precisa liberar a origem real do backend — self-hosted, então
// não tem domínio fixo conhecido de antemão (mesma razão do CORS por
// allowlist em vez de wildcard no backend). NEXT_PUBLIC_API_URL já é
// embutido no bundle no build (ver Dockerfile), então dá pra ler aqui e
// derivar tanto http(s):// (fetch normal) quanto ws(s):// (terminal).
const apiURL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:28080";
const apiOrigin = new URL(apiURL).origin;
const wsOrigin = apiOrigin.replace(/^http/, "ws");

// 'unsafe-eval' tirado (achado de auditoria) — React/Next.js não usam eval
// em produção (só em dev, pra reconstruir stack trace de erro do servidor
// no browser; ver node_modules/next/dist/docs/01-app/02-guides/
// content-security-policy.md), então isso não tem custo funcional aqui.
// 'unsafe-inline' em script-src continua — o CSP mais estrito possível
// (nonce por requisição via `proxy.ts`, API nova desse Next.js no lugar do
// antigo `middleware.ts`) força TODA página a renderização dinâmica (sem
// static optimization/ISR, ver mesmo doc acima), troca de arquitetura que
// vai além do que essa correção pontual deveria decidir sozinha — fica como
// trade-off aceito e documentado, não esquecido. O que já fecha sem esse
// custo: framing (frame-ancestors), injeção de <object>/<base>, exfiltração
// via fetch/WebSocket pra domínio desconhecido (connect-src restrito à
// própria origem + API), e agora eval também.
const csp = [
  "default-src 'self'",
  "script-src 'self' 'unsafe-inline'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self' data:",
  `connect-src 'self' ${apiOrigin} ${wsOrigin}`,
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'none'",
].join("; ");

const nextConfig: NextConfig = {
  output: "standalone",
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "Content-Security-Policy", value: csp },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "X-Content-Type-Options", value: "nosniff" },
        ],
      },
    ];
  },
};

export default nextConfig;
