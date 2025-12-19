-- +migrate Up
ALTER TABLE keys ADD COLUMN chain_code varchar(255);

-- +migrate Down
ALTER TABLE keys DROP COLUMN chain_code;
