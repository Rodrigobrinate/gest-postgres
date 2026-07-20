import type { NextConfig } from "next";

// connect-src precisa liberar a origem real do backend — self-hosted, então
// não tem domínio fixo conhecido de antemão (mesma razão do CORS por
// allowlist em vez de wildcard no backend). NEXT_PUBLIC_API_URL já é
// embutido no bundle no build (ver Dockerfile), então dá pra ler aqui e
// derivar tanto http(s):// (fetch normal) quanto ws(s):// (terminal).
const apiURL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:28080";
const apiOrigin = new URL(apiURL).origin;
const wsOrigin = apiOrigin.replace(/^http/, "ws");

// 'unsafe-inline'/'unsafe-eval' em script-src não é o CSP mais estrito
// possível (precisaria de nonce por requisição via middleware pra isso), mas
// fecha o que mais importa sem risco de quebrar libs que o app já usa
// (Recharts/CodeMirror/xterm) sem poder testar isso ao vivo num browser real
// nessa sessão: framing (frame-ancestors), injeção de <object>/<base>,
// exfiltração via fetch/WebSocket pra domínio desconhecido (connect-src
// restrito à própria origem + API).
const csp = [
  "default-src 'self'",
  "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
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
