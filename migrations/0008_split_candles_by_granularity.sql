-- +goose Up
-- +goose NO TRANSACTION

-- Create per-granularity candle tables with the same schema as the original.
CREATE TABLE IF NOT EXISTS candles_1m (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_1m_product_time ON candles_1m(product_id, time);

CREATE TABLE IF NOT EXISTS candles_5m (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_5m_product_time ON candles_5m(product_id, time);

CREATE TABLE IF NOT EXISTS candles_15m (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_15m_product_time ON candles_15m(product_id, time);

CREATE TABLE IF NOT EXISTS candles_30m (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_30m_product_time ON candles_30m(product_id, time);

CREATE TABLE IF NOT EXISTS candles_1h (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_1h_product_time ON candles_1h(product_id, time);

CREATE TABLE IF NOT EXISTS candles_2h (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_2h_product_time ON candles_2h(product_id, time);

CREATE TABLE IF NOT EXISTS candles_6h (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_6h_product_time ON candles_6h(product_id, time);

CREATE TABLE IF NOT EXISTS candles_1d (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_1d_product_time ON candles_1d(product_id, time);

-- Migrate existing data into candles_1m.
-- The old table has no granularity column. The bulk of the data comes from the
-- `history` command which hard-codes granularity to 1m. Moving everything to
-- candles_1m is the safest default; data fetched at other granularities can be
-- re-fetched with `cryptool exchange fetch --granularity <X>`.
INSERT INTO candles_1m SELECT * FROM candles;

DROP TABLE candles;

-- +goose Down
-- +goose NO TRANSACTION

-- Recreate the original unified candles table.
CREATE TABLE IF NOT EXISTS candles (
    exchange         TEXT             NOT NULL,
    product_id       TEXT             NOT NULL,
    time             TIMESTAMPTZ      NOT NULL,
    open             DOUBLE PRECISION NOT NULL,
    high             DOUBLE PRECISION NOT NULL,
    low              DOUBLE PRECISION NOT NULL,
    close            DOUBLE PRECISION NOT NULL,
    volume           DOUBLE PRECISION NOT NULL,
    fake_fill_count  INT              NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, product_id, time)
);
CREATE INDEX IF NOT EXISTS idx_candles_product_time ON candles(product_id, time);

-- Merge all per-granularity tables back. ON CONFLICT handles any overlapping timestamps.
INSERT INTO candles SELECT * FROM candles_1m ON CONFLICT DO NOTHING;
INSERT INTO candles SELECT * FROM candles_5m ON CONFLICT DO NOTHING;
INSERT INTO candles SELECT * FROM candles_15m ON CONFLICT DO NOTHING;
INSERT INTO candles SELECT * FROM candles_30m ON CONFLICT DO NOTHING;
INSERT INTO candles SELECT * FROM candles_1h ON CONFLICT DO NOTHING;
INSERT INTO candles SELECT * FROM candles_2h ON CONFLICT DO NOTHING;
INSERT INTO candles SELECT * FROM candles_6h ON CONFLICT DO NOTHING;
INSERT INTO candles SELECT * FROM candles_1d ON CONFLICT DO NOTHING;

DROP TABLE candles_1m;
DROP TABLE candles_5m;
DROP TABLE candles_15m;
DROP TABLE candles_30m;
DROP TABLE candles_1h;
DROP TABLE candles_2h;
DROP TABLE candles_6h;
DROP TABLE candles_1d;
