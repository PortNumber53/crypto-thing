-- +goose Up
CREATE TABLE IF NOT EXISTS wallets (
    exchange TEXT NOT NULL,
    uuid TEXT NOT NULL,
    name TEXT NOT NULL,
    currency TEXT NOT NULL,
    available_balance DOUBLE PRECISION NOT NULL,
    hold DOUBLE PRECISION NOT NULL,
    active BOOLEAN NOT NULL,
    "default" BOOLEAN NOT NULL,
    ready BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (exchange, uuid)
);

CREATE INDEX IF NOT EXISTS idx_wallets_exchange_currency ON wallets(exchange, currency);

-- +goose Down
DROP TABLE IF EXISTS wallets;
