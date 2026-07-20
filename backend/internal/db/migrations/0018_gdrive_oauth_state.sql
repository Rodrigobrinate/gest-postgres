-- state do OAuth do Google Drive era a constante fixa "gestpg", nunca
-- validada no callback — sem entropia nenhuma, não fornece proteção de CSRF
-- de vinculação de conta (ver relatório de segurança, item 8). Gerado
-- aleatório por chamada de GDriveAuthURL, guardado aqui, e o callback só
-- aceita se bater E não tiver expirado (uso único, curto prazo).
ALTER TABLE gdrive_connection
    ADD COLUMN oauth_state TEXT NOT NULL DEFAULT '',
    ADD COLUMN oauth_state_expires_at TIMESTAMPTZ;
