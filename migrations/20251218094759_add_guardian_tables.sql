-- +migrate Up
CREATE TABLE signing_policies (
    wallet_id varchar(255) PRIMARY KEY, -- 关联 KeyID
    policy_type varchar(50) NOT NULL, -- 'single' (单人), 'team' (团队多签)
    min_signatures int NOT NULL, -- 最小所需签名数
    created_at timestamp with time zone DEFAULT NOW(),
    updated_at timestamp with time zone DEFAULT NOW()
);

-- Passkey 存储表 (替代原有的 user_auth_keys)
CREATE TABLE user_passkeys (
    user_id varchar(255) NOT NULL,
    credential_id varchar(512) NOT NULL,
    public_key text NOT NULL, -- COSE Key Format (Hex/Base64)
    device_name varchar(255),
    created_at timestamp with time zone DEFAULT NOW(),
    PRIMARY KEY (user_id, credential_id)
);

-- +migrate Down
DROP TABLE user_passkeys;

DROP TABLE signing_policies;

