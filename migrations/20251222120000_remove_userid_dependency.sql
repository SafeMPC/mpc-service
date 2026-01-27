-- +migrate Up
ALTER TABLE IF EXISTS user_passkeys
    DROP CONSTRAINT IF EXISTS user_passkeys_pkey;

ALTER TABLE IF EXISTS user_passkeys
    DROP COLUMN IF EXISTS user_id;

ALTER TABLE IF EXISTS user_passkeys
    ADD PRIMARY KEY (credential_id);

ALTER TABLE backup_share_deliveries
    DROP COLUMN IF EXISTS user_id;

-- 更新 UNIQUE 约束（删除 user_id 后）
ALTER TABLE backup_share_deliveries
    DROP CONSTRAINT IF EXISTS backup_share_deliveries_key_id_node_id_share_index_user_id_key;

ALTER TABLE backup_share_deliveries
    ADD CONSTRAINT backup_share_deliveries_key_id_node_id_share_index_key UNIQUE (key_id, node_id, share_index);

-- 删除 user_id 索引（如果存在）
DROP INDEX IF EXISTS idx_backup_deliveries_user;

-- +migrate Down
-- 注意：这里无法简单恢复数据，仅为结构回滚
ALTER TABLE IF EXISTS user_passkeys
    ADD COLUMN IF NOT EXISTS user_id VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS user_passkeys
    DROP CONSTRAINT IF EXISTS user_passkeys_pkey;

ALTER TABLE IF EXISTS user_passkeys
    ADD PRIMARY KEY (user_id, credential_id);

ALTER TABLE backup_share_deliveries
    ADD COLUMN IF NOT EXISTS user_id VARCHAR(255);

-- 恢复 UNIQUE 约束（包含 user_id）
ALTER TABLE backup_share_deliveries
    DROP CONSTRAINT IF EXISTS backup_share_deliveries_key_id_node_id_share_index_key;

ALTER TABLE backup_share_deliveries
    ADD CONSTRAINT backup_share_deliveries_key_id_node_id_share_index_user_id_key UNIQUE (key_id, node_id, share_index, user_id);

-- 恢复 user_id 索引
CREATE INDEX IF NOT EXISTS idx_backup_deliveries_user ON backup_share_deliveries (user_id);

