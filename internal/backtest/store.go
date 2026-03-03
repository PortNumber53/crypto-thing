package backtest

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Candle is a local OHLCV candle used for backtesting.
type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// LoadCandles fetches real candles (volume >= 0) from the DB for a product/exchange over [start, end).
func LoadCandles(ctx context.Context, dbURL, exchange, product string, start, end time.Time) ([]Candle, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT time, open, high, low, close, volume
		FROM candles
		WHERE exchange = $1 AND product_id = $2
		  AND time >= $3 AND time < $4
		  AND volume >= 0
		ORDER BY time ASC
	`, exchange, product, start, end)
	if err != nil {
		return nil, fmt.Errorf("load candles: %w", err)
	}
	defer rows.Close()

	var candles []Candle
	for rows.Next() {
		var c Candle
		if err := rows.Scan(&c.Time, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err != nil {
			return nil, fmt.Errorf("scan candle: %w", err)
		}
		candles = append(candles, c)
	}
	return candles, rows.Err()
}

// SaveResult persists a backtest result to the DB.
func SaveResult(ctx context.Context, dbURL string, r Result) error {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		INSERT INTO backtest_results (
			exchange, product_id, granularity, start_time, end_time,
			signals, combination, threshold, fee_rate, slippage, tax_rate, min_edge, rr_min,
			total_return, sharpe, sortino, max_drawdown, calmar, win_rate, num_trades, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,NOW())
	`, r.Exchange, r.ProductID, r.Granularity, r.Start, r.End,
		r.Signals, r.Combination, r.Threshold, r.FeeRate, r.Slippage, r.TaxRate, r.MinEdge, r.RRMin,
		r.TotalReturn, r.Sharpe, r.Sortino, r.MaxDrawdown, r.Calmar, r.WinRate, r.NumTrades)
	return err
}
