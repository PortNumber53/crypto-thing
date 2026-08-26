-- +goose Up
ALTER TABLE backtest_results
    ADD COLUMN strategy_id BIGINT REFERENCES strategies(id) ON DELETE SET NULL,
    ADD COLUMN bull_signals TEXT NOT NULL DEFAULT '',
    ADD COLUMN bear_signals TEXT NOT NULL DEFAULT '',
    ADD COLUMN regime_fast INTEGER NOT NULL DEFAULT 50,
    ADD COLUMN regime_slow INTEGER NOT NULL DEFAULT 200,
    ADD COLUMN max_loss DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN profit_gate BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN position_size DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN capital DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN equity_curve JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN trades JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_backtest_results_strategy
    ON backtest_results(strategy_id, created_at DESC);

ALTER TABLE strategies
    ADD COLUMN bull_signals TEXT NOT NULL DEFAULT '',
    ADD COLUMN bear_signals TEXT NOT NULL DEFAULT '',
    ADD COLUMN regime_fast INTEGER NOT NULL DEFAULT 50,
    ADD COLUMN regime_slow INTEGER NOT NULL DEFAULT 200,
    ADD COLUMN max_loss DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN profit_gate BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN position_size DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN capital DOUBLE PRECISION NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE strategies
    DROP COLUMN capital,
    DROP COLUMN position_size,
    DROP COLUMN profit_gate,
    DROP COLUMN max_loss,
    DROP COLUMN regime_slow,
    DROP COLUMN regime_fast,
    DROP COLUMN bear_signals,
    DROP COLUMN bull_signals;

DROP INDEX IF EXISTS idx_backtest_results_strategy;

ALTER TABLE backtest_results
    DROP COLUMN trades,
    DROP COLUMN equity_curve,
    DROP COLUMN capital,
    DROP COLUMN position_size,
    DROP COLUMN profit_gate,
    DROP COLUMN max_loss,
    DROP COLUMN regime_slow,
    DROP COLUMN regime_fast,
    DROP COLUMN bear_signals,
    DROP COLUMN bull_signals,
    DROP COLUMN strategy_id;
