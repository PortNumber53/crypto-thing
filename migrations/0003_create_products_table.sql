-- +goose Up
CREATE TABLE IF NOT EXISTS products (
    exchange TEXT NOT NULL,
    product_id TEXT NOT NULL,
    base_name TEXT NOT NULL,
    quote_name TEXT NOT NULL,
    is_disabled BOOLEAN NOT NULL,
    PRIMARY KEY (exchange, product_id)
);

-- +goose Down
DROP TABLE IF EXISTS products;
