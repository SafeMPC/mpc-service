-- +migrate Up
CREATE TABLE user_auth_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    wallet_id varchar(255) NOT NULL, -- 关联的 MPC 钱包 ID (KeyID)
    public_key_hex varchar(512) NOT NULL, -- 用户公钥 (Hex 编码)
    key_type varchar(50) NOT NULL, -- 密钥类型: ed25519, secp256k1
    member_name varchar(100), -- 团队成员名称 (可选，用于审计)
    role VARCHAR(50), -- 角色: owner, admin, member
    created_at timestamp with time zone DEFAULT NOW(),
    updated_at timestamp with time zone DEFAULT NOW(),
    CONSTRAINT uk_wallet_pubkey UNIQUE (wallet_id, public_key_hex)
);

CREATE INDEX idx_user_auth_keys_wallet ON user_auth_keys (wallet_id);

CREATE TABLE signing_policies (
    wallet_id varchar(255) PRIMARY KEY, -- 关联 KeyID
    policy_type varchar(50) NOT NULL, -- 'single' (单人), 'team' (团队多签)
    min_signatures int NOT NULL, -- 最小所需签名数
    created_at timestamp with time zone DEFAULT NOW(),
    updated_at timestamp with time zone DEFAULT NOW()
);

-- +migrate Down
DROP TABLE signing_policies;

DROP TABLE user_auth_keys;

