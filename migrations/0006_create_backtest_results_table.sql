-- +goose Up
CREATE TABLE IF NOT EXISTS backtest_results (
    id              BIGSERIAL PRIMARY KEY,
    exchange        TEXT NOT NULL,
    product_id      TEXT NOT NULL,
    granularity     TEXT NOT NULL,
    start_time      TIMESTAMPTZ NOT NULL,
    end_time        TIMESTAMPTZ NOT NULL,
    signals         TEXT NOT NULL,
    combination     TEXT NOT NULL,
    threshold       DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    fee_rate        DOUBLE PRECISION NOT NULL DEFAULT 0.001,
    slippage        DOUBLE PRECISION NOT NULL DEFAULT 0.001,
    tax_rate        DOUBLE PRECISION NOT NULL DEFAULT 0.30,
    min_edge        DOUBLE PRECISION NOT NULL DEFAULT 0.005,
    rr_min          DOUBLE PRECISION NOT NULL DEFAULT 1.5,
    total_return    DOUBLE PRECISION NOT NULL,
    sharpe          DOUBLE PRECISION NOT NULL,
    sortino         DOUBLE PRECISION NOT NULL,
    max_drawdown    DOUBLE PRECISION NOT NULL,
    calmar          DOUBLE PRECISION NOT NULL,
    win_rate        DOUBLE PRECISION NOT NULL,
    num_trades      INTEGER NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_backtest_results_product ON backtest_results(exchange, product_id, granularity);
CREATE INDEX IF NOT EXISTS idx_backtest_results_created ON backtest_results(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS backtest_results;
