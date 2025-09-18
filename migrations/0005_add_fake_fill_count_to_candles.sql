-- +goose Up
ALTER TABLE candles ADD COLUMN fake_fill_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE candles DROP COLUMN fake_fill_count;
