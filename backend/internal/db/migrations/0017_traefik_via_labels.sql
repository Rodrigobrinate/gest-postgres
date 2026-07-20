-- Rota via LABEL Docker em vez de arquivo de config do file provider —
-- usada quando já existe um Traefik externo (ex: EasyPanel) no host, pra
-- nunca precisar recriar/tocar o Traefik alheio: só o container ALVO é
-- recriado com os labels, o Traefik externo (com provider Docker, padrão
-- dele) descobre a rota sozinho. Reaproveita target_container/target_port
-- que já existiam — via_labels só muda o MECANISMO de aplicar, não o
-- schema de "pra onde aponta".
ALTER TABLE proxy_routes
    ADD COLUMN via_labels BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN cert_resolver TEXT NOT NULL DEFAULT '';
