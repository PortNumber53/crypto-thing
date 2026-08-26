package backtest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"cryptool/internal/schema"
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
func LoadCandles(ctx context.Context, dbURL, exchange, product, granularity string, start, end time.Time) ([]Candle, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	table := schema.CandleTable(granularity)

	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT time, open, high, low, close, volume
		FROM %s
		WHERE exchange = $1 AND product_id = $2
		  AND time >= $3 AND time < $4
		  AND volume >= 0
		ORDER BY time ASC
	`, table), exchange, product, start, end)
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

type storedTrade struct {
	EntryTime  time.Time `json:"entry_time"`
	ExitTime   time.Time `json:"exit_time"`
	Direction  string    `json:"direction"`
	EntryPrice float64   `json:"entry_price"`
	ExitPrice  float64   `json:"exit_price"`
	NetReturn  float64   `json:"net_return"`
	Profit     bool      `json:"profit"`
}

type storedEquityPoint struct {
	Time   time.Time `json:"time"`
	Equity float64   `json:"equity"`
	Price  float64   `json:"price"`
}

// SaveResult persists a backtest result and fills its database identity.
func SaveResult(ctx context.Context, dbURL string, r *Result) error {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	trades := make([]storedTrade, len(r.Trades))
	for i, trade := range r.Trades {
		direction := "long"
		if trade.Direction == -1 {
			direction = "short"
		}
		trades[i] = storedTrade{
			EntryTime: trade.EntryTime, ExitTime: trade.ExitTime, Direction: direction,
			EntryPrice: trade.EntryPrice, ExitPrice: trade.ExitPrice,
			NetReturn: trade.NetReturn, Profit: trade.Profit,
		}
	}
	equityPoints := SampleEquityCurve(r.EquityCurve, 2000)
	equity := make([]storedEquityPoint, len(equityPoints))
	for i, point := range equityPoints {
		equity[i] = storedEquityPoint(point)
	}
	tradeJSON, err := json.Marshal(trades)
	if err != nil {
		return fmt.Errorf("marshal trades: %w", err)
	}
	equityJSON, err := json.Marshal(equity)
	if err != nil {
		return fmt.Errorf("marshal equity curve: %w", err)
	}

	err = db.QueryRowContext(ctx, `
		INSERT INTO backtest_results (
			strategy_id, exchange, product_id, granularity, start_time, end_time,
			signals, bull_signals, bear_signals, regime_fast, regime_slow,
			combination, threshold, fee_rate, slippage, tax_rate, min_edge, rr_min,
			max_loss, profit_gate, position_size, capital,
			total_return, sharpe, sortino, max_drawdown, calmar, win_rate, num_trades,
			equity_curve, trades, created_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
			$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,NOW()
		) RETURNING id, created_at
	`, r.StrategyID, r.Exchange, r.ProductID, r.Granularity, r.Start, r.End,
		r.Signals, r.BullSignals, r.BearSignals, r.RegimeFast, r.RegimeSlow,
		r.Combination, r.Threshold, r.FeeRate, r.Slippage, r.TaxRate, r.MinEdge, r.RRMin,
		r.MaxLoss, r.ProfitGate, r.PositionSize, r.Capital,
		r.TotalReturn, r.Sharpe, r.Sortino, r.MaxDrawdown, r.Calmar, r.WinRate, r.NumTrades,
		equityJSON, tradeJSON).Scan(&r.ID, &r.CreatedAt)
	return err
}
