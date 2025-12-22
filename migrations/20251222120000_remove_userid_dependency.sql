-- +migrate Up
ALTER TABLE IF EXISTS user_passkeys
    DROP CONSTRAINT IF EXISTS user_passkeys_pkey;

ALTER TABLE IF EXISTS user_passkeys
    DROP COLUMN IF EXISTS user_id;

ALTER TABLE IF EXISTS user_passkeys
    ADD PRIMARY KEY (credential_id);

ALTER TABLE backup_share_deliveries
    DROP COLUMN user_id;

-- +migrate Down
-- 注意：这里无法简单恢复数据，仅为结构回滚
ALTER TABLE IF EXISTS user_passkeys
    ADD COLUMN IF NOT EXISTS user_id VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS user_passkeys
    DROP CONSTRAINT IF EXISTS user_passkeys_pkey;

ALTER TABLE IF EXISTS user_passkeys
    ADD PRIMARY KEY (user_id, credential_id);

ALTER TABLE backup_share_deliveries
    ADD COLUMN user_id VARCHAR(255);

