-- +goose Up
CREATE TABLE IF NOT EXISTS strategies (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    signals     TEXT NOT NULL DEFAULT 'rsi,macd,bbands,ema_cross',
    combination TEXT NOT NULL DEFAULT 'voting',
    threshold   DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    fee_rate    DOUBLE PRECISION NOT NULL DEFAULT 0.001,
    slippage    DOUBLE PRECISION NOT NULL DEFAULT 0.001,
    tax_rate    DOUBLE PRECISION NOT NULL DEFAULT 0.30,
    min_edge    DOUBLE PRECISION NOT NULL DEFAULT 0.005,
    rr_min      DOUBLE PRECISION NOT NULL DEFAULT 1.5,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS strategies;
