-- +migrate Up
ALTER TABLE IF EXISTS user_passkeys RENAME TO passkeys;

ALTER TABLE IF EXISTS passkeys
    DROP CONSTRAINT IF EXISTS user_passkeys_pkey;

ALTER TABLE IF EXISTS passkeys
    DROP CONSTRAINT IF EXISTS passkeys_pkey;

ALTER TABLE IF EXISTS passkeys
    ADD PRIMARY KEY (credential_id);

-- +migrate Down
ALTER TABLE IF EXISTS passkeys RENAME TO user_passkeys;

ALTER TABLE IF EXISTS user_passkeys
    DROP CONSTRAINT IF EXISTS passkeys_pkey;

ALTER TABLE IF EXISTS user_passkeys
    DROP CONSTRAINT IF EXISTS user_passkeys_pkey;

ALTER TABLE IF EXISTS user_passkeys
    ADD PRIMARY KEY (credential_id);

