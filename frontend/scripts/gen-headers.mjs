// Gera public/_headers (formato Cloudflare Pages) a partir de
// NEXT_PUBLIC_API_URL — roda ANTES de `next build` (ver package.json). CSP
// precisa saber a origem real da API em build time (mesma info que já
// embute no bundle JS via NEXT_PUBLIC_API_URL), mas headers() do Next não
// existe mais em output "export" (ver next.config.ts), então esse é o
// equivalente estático: Cloudflare Pages lê public/_headers e aplica em
// toda resposta automaticamente, sem processamento nenhum do lado dele.
import { writeFileSync, mkdirSync } from "node:fs";

const apiURL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:28080";
const apiOrigin = new URL(apiURL).origin;
const wsOrigin = apiOrigin.replace(/^http/, "ws");

// 'unsafe-eval' fora, 'unsafe-inline' em script-src aceito como trade-off —
// mesmo racional documentado antes em next.config.ts (nonce por requisição
// exigiria middleware/edge function, fora de alcance de output estático).
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

const content = `/*
  Content-Security-Policy: ${csp}
  X-Frame-Options: DENY
  X-Content-Type-Options: nosniff
`;

mkdirSync(new URL("../public", import.meta.url), { recursive: true });
writeFileSync(new URL("../public/_headers", import.meta.url), content);
console.log(`public/_headers gerado (API: ${apiOrigin})`);
