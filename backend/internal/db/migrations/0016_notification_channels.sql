-- Canal de notificação reutilizável (Telegram ou webhook genérico) —
-- configurado uma vez, referenciado por qualquer regra de alerta em vez de
-- colar a mesma URL/token em cada regra separadamente. telegram_bot_token
-- cifrado com o mesmo internal/crypto.SecretBox de sempre.
CREATE TABLE IF NOT EXISTS notification_channels (
    id                           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                         TEXT NOT NULL UNIQUE,
    kind                         TEXT NOT NULL CHECK (kind IN ('telegram', 'webhook')),
    webhook_url                  TEXT NOT NULL DEFAULT '',
    telegram_bot_token_encrypted TEXT NOT NULL DEFAULT '',
    telegram_chat_id             TEXT NOT NULL DEFAULT '',
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Regra de alerta passa a poder referenciar um canal salvo em vez de
-- (ou além de) uma webhook_url direta colada na hora — webhook_url
-- continua existindo pra regra antiga / uso avulso sem cadastrar canal.
ALTER TABLE alert_rules
    ADD COLUMN channel_id UUID REFERENCES notification_channels (id) ON DELETE SET NULL;
ALTER TABLE alert_rules ALTER COLUMN webhook_url DROP NOT NULL;
ALTER TABLE alert_rules ALTER COLUMN webhook_url SET DEFAULT '';
UPDATE alert_rules SET webhook_url = '' WHERE webhook_url IS NULL;
ALTER TABLE alert_rules ALTER COLUMN webhook_url SET NOT NULL;
