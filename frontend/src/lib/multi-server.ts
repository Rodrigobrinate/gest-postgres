// MULTI_SERVER_MODE liga o modo "hospedado no Cloudflare Pages, atrás do
// Worker do sistema mestre" — build-time, nunca runtime. Falso por padrão:
// build normal (docker-compose --profile with-frontend, 1 frontend por
// droplet, API_URL = backend direto) continua se comportando exatamente
// como sempre, sem overview/seleção de instalação nem prefixo /proxy/*
// nas chamadas. Só builds feitos pro Cloudflare Pages (fora do
// docker-compose, ver CI) setam essa env var.
export const MULTI_SERVER_MODE = process.env.NEXT_PUBLIC_MULTI_SERVER_MODE === "1";
