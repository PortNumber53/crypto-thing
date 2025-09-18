-- +goose Up
CREATE TABLE IF NOT EXISTS candles (
    exchange   TEXT       NOT NULL,
    product_id TEXT       NOT NULL,
    time       TIMESTAMPTZ NOT NULL,
    open       DOUBLE PRECISION NOT NULL,
    high       DOUBLE PRECISION NOT NULL,
    low        DOUBLE PRECISION NOT NULL,
    close      DOUBLE PRECISION NOT NULL,
    volume     DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (exchange, product_id, time)
);

CREATE INDEX IF NOT EXISTS idx_candles_product_time ON candles(product_id, time);

-- +goose Down
DROP TABLE IF EXISTS candles;
