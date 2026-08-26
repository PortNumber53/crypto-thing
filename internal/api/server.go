package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cryptool/internal/backtest"
	"cryptool/internal/coinbase"
	"cryptool/internal/config"
	"cryptool/internal/ingest"
	"cryptool/internal/schema"

	_ "github.com/lib/pq"
)

// Server is the HTTP API server.
type Server struct {
	db        *sql.DB
	cfg       *config.Config
	backtests backtest.Runner
}

// NewServer opens the DB and returns a ready server.
func NewServer(cfg *config.Config) (*Server, error) {
	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return &Server{db: db, cfg: cfg, backtests: backtest.NewService(cfg.Database.URL)}, nil
}

// Close releases the DB connection pool.
func (s *Server) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

// Handler returns the root http.Handler with all routes and CORS.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/stats", s.handleStats)

	mux.HandleFunc("GET /api/products", s.handleListProducts)
	mux.HandleFunc("POST /api/products/sync", s.handleSyncProducts)

	mux.HandleFunc("GET /api/candles", s.handleCandles)

	mux.HandleFunc("GET /api/signals", s.handleSignals)

	mux.HandleFunc("GET /api/backtest/results", s.handleListBacktests)
	mux.HandleFunc("GET /api/backtest/results/{id}", s.handleGetBacktest)
	mux.HandleFunc("POST /api/backtest/run", s.handleRunBacktest)
	mux.HandleFunc("POST /api/backtest/combo", s.handleRunBacktestCombinations)
	mux.HandleFunc("DELETE /api/backtest/results/{id}", s.handleDeleteBacktest)

	mux.HandleFunc("GET /api/strategies", s.handleListStrategies)
	mux.HandleFunc("POST /api/strategies", s.handleCreateStrategy)
	mux.HandleFunc("GET /api/strategies/{id}", s.handleGetStrategy)
	mux.HandleFunc("PUT /api/strategies/{id}", s.handleUpdateStrategy)
	mux.HandleFunc("DELETE /api/strategies/{id}", s.handleDeleteStrategy)
	mux.HandleFunc("POST /api/strategies/{id}/run", s.handleRunStrategy)

	return corsMiddleware(mux)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date %q", s)
}

// ─── /api/health ────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := s.db.PingContext(r.Context()); err != nil {
		dbStatus = err.Error()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": dbStatus})
}

// ─── /api/stats ─────────────────────────────────────────────────────────────

type StatsResponse struct {
	Products        int                 `json:"products"`
	Candles         int64               `json:"candles"`
	Backtests       int                 `json:"backtests"`
	Strategies      int                 `json:"strategies"`
	BestBacktest    *BacktestResultRow  `json:"best_backtest"`
	RecentBacktests []BacktestResultRow `json:"recent_backtests"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	var stats StatsResponse
	row := s.db.QueryRowContext(r.Context(), `
		SELECT
			(SELECT COUNT(*) FROM products) AS products,
			(SELECT COALESCE(SUM(cnt), 0) FROM (
				SELECT COUNT(*) AS cnt FROM candles_1m WHERE volume >= 0
				UNION ALL SELECT COUNT(*) FROM candles_5m WHERE volume >= 0
				UNION ALL SELECT COUNT(*) FROM candles_15m WHERE volume >= 0
				UNION ALL SELECT COUNT(*) FROM candles_30m WHERE volume >= 0
				UNION ALL SELECT COUNT(*) FROM candles_1h WHERE volume >= 0
				UNION ALL SELECT COUNT(*) FROM candles_2h WHERE volume >= 0
				UNION ALL SELECT COUNT(*) FROM candles_6h WHERE volume >= 0
				UNION ALL SELECT COUNT(*) FROM candles_1d WHERE volume >= 0
			) sub) AS candles,
			(SELECT COUNT(*) FROM backtest_results) AS backtests,
			(SELECT COUNT(*) FROM strategies) AS strategies
	`)
	if err := row.Scan(&stats.Products, &stats.Candles, &stats.Backtests, &stats.Strategies); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	best, err := scanBacktestRow(s.db.QueryRowContext(r.Context(),
		`SELECT`+backtestSelectCols+` FROM backtest_results ORDER BY total_return DESC, created_at DESC LIMIT 1`))
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err == nil {
		best.EquityCurve, best.Trades = nil, nil
		stats.BestBacktest = &best
	}
	recentRows, err := s.db.QueryContext(r.Context(),
		`SELECT`+backtestSelectCols+` FROM backtest_results ORDER BY created_at DESC LIMIT 5`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer recentRows.Close()
	stats.RecentBacktests = []BacktestResultRow{}
	for recentRows.Next() {
		item, scanErr := scanBacktestRow(recentRows)
		if scanErr != nil {
			writeError(w, http.StatusInternalServerError, scanErr.Error())
			return
		}
		item.EquityCurve, item.Trades = nil, nil
		stats.RecentBacktests = append(stats.RecentBacktests, item)
	}
	writeJSON(w, http.StatusOK, stats)
}

// ─── /api/products ──────────────────────────────────────────────────────────

type ProductResponse struct {
	Exchange        string  `json:"exchange"`
	ProductID       string  `json:"product_id"`
	DisplayName     string  `json:"display_name"`
	BaseCurrencyID  string  `json:"base_currency_id"`
	QuoteCurrencyID string  `json:"quote_currency_id"`
	Price           float64 `json:"price"`
	PriceChange24h  float64 `json:"price_change_24h"`
	Volume24h       float64 `json:"volume_24h"`
	Status          string  `json:"status"`
	Watched         bool    `json:"watched"`
	NewAt           *string `json:"new_at"`
	CandleCount     int64   `json:"candle_count"`
}

func (s *Server) handleListProducts(w http.ResponseWriter, r *http.Request) {
	exchange := r.URL.Query().Get("exchange")
	if exchange == "" {
		exchange = "coinbase"
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))

	var (
		query string
		args  []any
	)
	if search != "" {
		pattern := "%" + strings.ToUpper(search) + "%"
		query = `
			SELECT p.exchange, p.product_id,
				COALESCE(p.display_name, p.product_id) AS display_name,
				COALESCE(p.base_currency_id, '') AS base_currency_id,
				COALESCE(p.quote_currency_id, '') AS quote_currency_id,
				COALESCE(p.price, 0),
				COALESCE(p.price_percentage_change_24h, 0),
				COALESCE(p.volume_24h, 0),
				COALESCE(p.status, ''),
				COALESCE(p.watched, false),
				p.new_at,
				COUNT(c.time) FILTER (WHERE c.volume >= 0) AS candle_count
			FROM products p
			LEFT JOIN candles_1h c ON c.exchange = p.exchange AND c.product_id = p.product_id
			WHERE p.exchange = $1
			  AND (UPPER(p.product_id) LIKE $2 OR UPPER(COALESCE(p.display_name,'')) LIKE $2)
			GROUP BY p.exchange, p.product_id, p.display_name, p.base_currency_id,
				p.quote_currency_id, p.price, p.price_percentage_change_24h,
				p.volume_24h, p.status, p.watched, p.new_at
			ORDER BY p.volume_24h DESC NULLS LAST
			LIMIT 200`
		args = []any{exchange, pattern}
	} else {
		query = `
			SELECT p.exchange, p.product_id,
				COALESCE(p.display_name, p.product_id) AS display_name,
				COALESCE(p.base_currency_id, '') AS base_currency_id,
				COALESCE(p.quote_currency_id, '') AS quote_currency_id,
				COALESCE(p.price, 0),
				COALESCE(p.price_percentage_change_24h, 0),
				COALESCE(p.volume_24h, 0),
				COALESCE(p.status, ''),
				COALESCE(p.watched, false),
				p.new_at,
				COUNT(c.time) FILTER (WHERE c.volume >= 0) AS candle_count
			FROM products p
			LEFT JOIN candles_1h c ON c.exchange = p.exchange AND c.product_id = p.product_id
			WHERE p.exchange = $1
			GROUP BY p.exchange, p.product_id, p.display_name, p.base_currency_id,
				p.quote_currency_id, p.price, p.price_percentage_change_24h,
				p.volume_24h, p.status, p.watched, p.new_at
			ORDER BY p.volume_24h DESC NULLS LAST
			LIMIT 200`
		args = []any{exchange}
	}

	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var products []ProductResponse
	for rows.Next() {
		var p ProductResponse
		var newAt sql.NullTime
		if err := rows.Scan(
			&p.Exchange, &p.ProductID, &p.DisplayName,
			&p.BaseCurrencyID, &p.QuoteCurrencyID,
			&p.Price, &p.PriceChange24h, &p.Volume24h,
			&p.Status, &p.Watched, &newAt, &p.CandleCount,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if newAt.Valid {
			s := newAt.Time.Format(time.RFC3339)
			p.NewAt = &s
		}
		products = append(products, p)
	}
	if products == nil {
		products = []ProductResponse{}
	}
	writeJSON(w, http.StatusOK, products)
}

func (s *Server) handleSyncProducts(w http.ResponseWriter, r *http.Request) {
	var client *coinbase.Client
	var err error
	if s.cfg.Coinbase.APIKeyName != "" && s.cfg.Coinbase.APIPrivateKey != "" {
		client, err = coinbase.NewClientWithJWT(s.cfg.Coinbase.APIKeyName, s.cfg.Coinbase.APIPrivateKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "jwt client: "+err.Error())
			return
		}
	} else {
		client = coinbase.NewClient(s.cfg.Coinbase.APIKey, s.cfg.Coinbase.APISecret, s.cfg.Coinbase.Passphrase)
	}
	client.Configure(s.cfg.Coinbase.RPM, s.cfg.Coinbase.MaxRetries, s.cfg.Coinbase.BackoffMS, false)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	products, err := client.GetProducts(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "coinbase: "+err.Error())
		return
	}
	store := ingest.NewStore(s.cfg.Database.URL)
	n, err := store.UpsertProducts(ctx, "coinbase", products)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"synced": n, "total": len(products)})
}

// ─── /api/candles ───────────────────────────────────────────────────────────

type CandleResponse struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

func (s *Server) handleCandles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	exchange := q.Get("exchange")
	if exchange == "" {
		exchange = "coinbase"
	}
	product := q.Get("product")
	if product == "" {
		writeError(w, http.StatusBadRequest, "product is required")
		return
	}
	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = "1h"
	}

	start, _ := parseDate(q.Get("start"))
	end, _ := parseDate(q.Get("end"))
	if start.IsZero() {
		start = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}

	limit := 500
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 5000 {
		limit = 5000
	}

	table := schema.CandleTable(granularity)
	rows, err := s.db.QueryContext(r.Context(), fmt.Sprintf(`
		SELECT time, open, high, low, close, volume
		FROM %s
		WHERE exchange = $1 AND product_id = $2
		  AND time >= $3 AND time < $4
		  AND volume >= 0
		ORDER BY time ASC
		LIMIT $5
	`, table), exchange, product, start, end, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var candles []CandleResponse
	for rows.Next() {
		var c CandleResponse
		var t time.Time
		if err := rows.Scan(&t, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		c.Time = t.UTC().Format(time.RFC3339)
		candles = append(candles, c)
	}
	if candles == nil {
		candles = []CandleResponse{}
	}
	writeJSON(w, http.StatusOK, candles)
}

// ─── /api/signals ───────────────────────────────────────────────────────────

type SignalInfo struct {
	Name        string         `json:"name"`
	Display     string         `json:"display"`
	Description string         `json:"description"`
	Params      map[string]any `json:"params"`
}

func (s *Server) handleSignals(w http.ResponseWriter, r *http.Request) {
	signals := []SignalInfo{
		{
			Name:        "rsi",
			Display:     "RSI",
			Description: "Relative Strength Index. Buys when RSI < 30 (oversold), sells when RSI > 70 (overbought). Measures momentum on a 0–100 scale.",
			Params:      map[string]any{"period": 14, "buy_threshold": 30, "sell_threshold": 70},
		},
		{
			Name:        "macd",
			Display:     "MACD",
			Description: "Moving Average Convergence/Divergence. Generates a buy signal when the MACD line crosses above the signal line, and a sell when it crosses below.",
			Params:      map[string]any{"fast": 12, "slow": 26, "signal": 9},
		},
		{
			Name:        "bbands",
			Display:     "Bollinger Bands",
			Description: "Buys when price closes below the lower band (oversold volatility squeeze), sells when price closes above the upper band.",
			Params:      map[string]any{"period": 20, "std_dev": 2.0},
		},
		{
			Name:        "ema_cross",
			Display:     "EMA Crossover",
			Description: "Exponential Moving Average crossover. Buys when the fast EMA (9) crosses above the slow EMA (21), indicating a bullish trend shift.",
			Params:      map[string]any{"fast": 9, "slow": 21},
		},
	}
	signals = append(signals, SignalInfo{
		Name:        "sma",
		Display:     "SMA Cross",
		Description: "Simple Moving Average crossover. Buys when price crosses above the SMA, sells when price crosses below. Default period is 200 (the classic long-term trend filter).",
		Params:      map[string]any{"period": 200},
	})
	writeJSON(w, http.StatusOK, signals)
}

// ─── /api/backtest ──────────────────────────────────────────────────────────

type BacktestResultRow struct {
	ID           int64             `json:"id"`
	StrategyID   *int64            `json:"strategy_id"`
	Exchange     string            `json:"exchange"`
	ProductID    string            `json:"product_id"`
	Granularity  string            `json:"granularity"`
	StartTime    string            `json:"start_time"`
	EndTime      string            `json:"end_time"`
	Signals      string            `json:"signals"`
	BullSignals  string            `json:"bull_signals"`
	BearSignals  string            `json:"bear_signals"`
	RegimeFast   int               `json:"regime_fast"`
	RegimeSlow   int               `json:"regime_slow"`
	Combination  string            `json:"combination"`
	Threshold    float64           `json:"threshold"`
	FeeRate      float64           `json:"fee_rate"`
	Slippage     float64           `json:"slippage"`
	TaxRate      float64           `json:"tax_rate"`
	MinEdge      float64           `json:"min_edge"`
	RRMin        float64           `json:"rr_min"`
	MaxLoss      float64           `json:"max_loss"`
	ProfitGate   bool              `json:"profit_gate"`
	PositionSize float64           `json:"position_size"`
	Capital      float64           `json:"capital"`
	TotalReturn  float64           `json:"total_return"`
	Sharpe       float64           `json:"sharpe"`
	Sortino      float64           `json:"sortino"`
	MaxDrawdown  float64           `json:"max_drawdown"`
	Calmar       float64           `json:"calmar"`
	WinRate      float64           `json:"win_rate"`
	NumTrades    int               `json:"num_trades"`
	CreatedAt    string            `json:"created_at"`
	EquityCurve  []EquityPointJSON `json:"equity_curve,omitempty"`
	Trades       []TradeJSON       `json:"trades,omitempty"`
}

type EquityPointJSON struct {
	Time   time.Time `json:"time"`
	Equity float64   `json:"equity"`
	Price  float64   `json:"price"`
}

type TradeJSON struct {
	EntryTime  time.Time `json:"entry_time"`
	ExitTime   time.Time `json:"exit_time"`
	Direction  string    `json:"direction"`
	EntryPrice float64   `json:"entry_price"`
	ExitPrice  float64   `json:"exit_price"`
	NetReturn  float64   `json:"net_return"`
	Profit     bool      `json:"profit"`
}

func scanBacktestRow(rows interface {
	Scan(...any) error
}) (BacktestResultRow, error) {
	var r BacktestResultRow
	var start, end, created time.Time
	var equityJSON, tradesJSON []byte
	err := rows.Scan(
		&r.ID, &r.StrategyID, &r.Exchange, &r.ProductID, &r.Granularity,
		&start, &end,
		&r.Signals, &r.BullSignals, &r.BearSignals, &r.RegimeFast, &r.RegimeSlow,
		&r.Combination, &r.Threshold,
		&r.FeeRate, &r.Slippage, &r.TaxRate, &r.MinEdge, &r.RRMin,
		&r.MaxLoss, &r.ProfitGate, &r.PositionSize, &r.Capital,
		&r.TotalReturn, &r.Sharpe, &r.Sortino, &r.MaxDrawdown,
		&r.Calmar, &r.WinRate, &r.NumTrades, &equityJSON, &tradesJSON, &created,
	)
	if err != nil {
		return r, err
	}
	r.StartTime = start.Format(time.RFC3339)
	r.EndTime = end.Format(time.RFC3339)
	r.CreatedAt = created.Format(time.RFC3339)
	if err := json.Unmarshal(equityJSON, &r.EquityCurve); err != nil {
		return r, fmt.Errorf("decode equity curve: %w", err)
	}
	if err := json.Unmarshal(tradesJSON, &r.Trades); err != nil {
		return r, fmt.Errorf("decode trades: %w", err)
	}
	return r, nil
}

const backtestSelectCols = `
	id, strategy_id, exchange, product_id, granularity, start_time, end_time,
	signals, bull_signals, bear_signals, regime_fast, regime_slow,
	combination, threshold, fee_rate, slippage, tax_rate, min_edge, rr_min,
	max_loss, profit_gate, position_size, capital,
	total_return, sharpe, sortino, max_drawdown, calmar, win_rate, num_trades,
	equity_curve, trades, created_at`

func (s *Server) handleListBacktests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	product := q.Get("product")
	strategyID := q.Get("strategy_id")
	limit := 50
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	var rows *sql.Rows
	var err error
	if strategyID != "" {
		rows, err = s.db.QueryContext(r.Context(),
			`SELECT`+backtestSelectCols+` FROM backtest_results WHERE strategy_id = $1 ORDER BY created_at DESC LIMIT $2`,
			strategyID, limit)
	} else if product != "" {
		rows, err = s.db.QueryContext(r.Context(),
			`SELECT`+backtestSelectCols+` FROM backtest_results WHERE product_id = $1 ORDER BY created_at DESC LIMIT $2`,
			product, limit)
	} else {
		rows, err = s.db.QueryContext(r.Context(),
			`SELECT`+backtestSelectCols+` FROM backtest_results ORDER BY created_at DESC LIMIT $1`,
			limit)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var results []BacktestResultRow
	for rows.Next() {
		br, err := scanBacktestRow(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		br.EquityCurve, br.Trades = nil, nil
		results = append(results, br)
	}
	if results == nil {
		results = []BacktestResultRow{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleGetBacktest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	row := s.db.QueryRowContext(r.Context(),
		`SELECT`+backtestSelectCols+` FROM backtest_results WHERE id = $1`, id)
	br, err := scanBacktestRow(row)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, br)
}

func (s *Server) handleDeleteBacktest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM backtest_results WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type RunBacktestRequest struct {
	Exchange     string   `json:"exchange"`
	ProductID    string   `json:"product_id"`
	Granularity  string   `json:"granularity"`
	Start        string   `json:"start"`
	End          string   `json:"end"`
	Signals      []string `json:"signals"`
	BullSignals  []string `json:"bull_signals"`
	BearSignals  []string `json:"bear_signals"`
	Combination  string   `json:"combination"`
	RegimeFast   int      `json:"regime_fast"`
	RegimeSlow   int      `json:"regime_slow"`
	Threshold    float64  `json:"threshold"`
	FeeRate      float64  `json:"fee_rate"`
	Slippage     float64  `json:"slippage"`
	TaxRate      float64  `json:"tax_rate"`
	MinEdge      float64  `json:"min_edge"`
	RRMin        float64  `json:"rr_min"`
	MaxLoss      float64  `json:"max_loss"`
	ProfitGate   bool     `json:"profit_gate"`
	PositionSize float64  `json:"position_size"`
	Capital      float64  `json:"capital"`
	Save         bool     `json:"save"`
	Top          int      `json:"top,omitempty"`
}

func defaultRunBacktestRequest() RunBacktestRequest {
	d := backtest.DefaultParams()
	return RunBacktestRequest{
		Exchange: d.Exchange, Granularity: d.Granularity, Combination: string(d.Combination),
		RegimeFast: d.RegimeFast, RegimeSlow: d.RegimeSlow, Threshold: d.Threshold,
		FeeRate: d.FeeRate, Slippage: d.Slippage, TaxRate: d.TaxRate,
		MinEdge: d.MinEdge, RRMin: d.RRMin, Top: 10,
	}
}

func decodeRunBacktestRequest(r *http.Request) (RunBacktestRequest, backtest.RunRequest, error) {
	req := defaultRunBacktestRequest()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, backtest.RunRequest{}, fmt.Errorf("invalid body: %w", err)
	}
	if req.ProductID == "" {
		return req, backtest.RunRequest{}, fmt.Errorf("product_id required")
	}
	start, err := parseDate(req.Start)
	if err != nil {
		return req, backtest.RunRequest{}, err
	}
	end, err := parseDate(req.End)
	if err != nil {
		return req, backtest.RunRequest{}, err
	}
	if start.IsZero() {
		start = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if !end.After(start) {
		return req, backtest.RunRequest{}, fmt.Errorf("end must be after start")
	}
	params := backtest.Params{
		Exchange:     req.Exchange,
		ProductID:    req.ProductID,
		Granularity:  req.Granularity,
		Start:        start,
		End:          end,
		Signals:      req.Signals,
		BullSignals:  req.BullSignals,
		BearSignals:  req.BearSignals,
		Combination:  backtest.CombinationMethod(req.Combination),
		RegimeFast:   req.RegimeFast,
		RegimeSlow:   req.RegimeSlow,
		Threshold:    req.Threshold,
		FeeRate:      req.FeeRate,
		Slippage:     req.Slippage,
		TaxRate:      req.TaxRate,
		MinEdge:      req.MinEdge,
		RRMin:        req.RRMin,
		MaxLoss:      req.MaxLoss,
		ProfitGate:   req.ProfitGate,
		PositionSize: req.PositionSize,
		Capital:      req.Capital,
	}
	if err := params.Validate(); err != nil {
		return req, backtest.RunRequest{}, err
	}
	return req, backtest.RunRequest{Params: params, Save: req.Save}, nil
}

func (s *Server) handleRunBacktest(w http.ResponseWriter, r *http.Request) {
	_, request, err := decodeRunBacktestRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.backtests.Run(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, backtestResultToRow(result))
}

func (s *Server) handleRunBacktestCombinations(w http.ResponseWriter, r *http.Request) {
	req, request, err := decodeRunBacktestRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	top := req.Top
	if value := r.URL.Query().Get("top"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < 1 || parsed > 31 {
			writeError(w, http.StatusBadRequest, "top must be between 1 and 31")
			return
		}
		top = parsed
	}
	if top < 1 || top > 31 {
		writeError(w, http.StatusBadRequest, "top must be between 1 and 31")
		return
	}
	results, err := s.backtests.RunCombinations(r.Context(), backtest.CombinationRequest{RunRequest: request, Top: top})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	rows := make([]BacktestResultRow, len(results))
	for i, result := range results {
		rows[i] = backtestResultToRow(result)
		rows[i].EquityCurve = nil
		rows[i].Trades = nil
	}
	writeJSON(w, http.StatusOK, rows)
}

func backtestResultToRow(r backtest.Result) BacktestResultRow {
	return BacktestResultRow{
		ID:           r.ID,
		StrategyID:   r.StrategyID,
		Exchange:     r.Exchange,
		ProductID:    r.ProductID,
		Granularity:  r.Granularity,
		StartTime:    r.Start.Format(time.RFC3339),
		EndTime:      r.End.Format(time.RFC3339),
		Signals:      r.Signals,
		BullSignals:  r.BullSignals,
		BearSignals:  r.BearSignals,
		RegimeFast:   r.RegimeFast,
		RegimeSlow:   r.RegimeSlow,
		Combination:  r.Combination,
		Threshold:    r.Threshold,
		FeeRate:      r.FeeRate,
		Slippage:     r.Slippage,
		TaxRate:      r.TaxRate,
		MinEdge:      r.MinEdge,
		RRMin:        r.RRMin,
		MaxLoss:      r.MaxLoss,
		ProfitGate:   r.ProfitGate,
		PositionSize: r.PositionSize,
		Capital:      r.Capital,
		TotalReturn:  r.TotalReturn,
		Sharpe:       r.Sharpe,
		Sortino:      r.Sortino,
		MaxDrawdown:  r.MaxDrawdown,
		Calmar:       r.Calmar,
		WinRate:      r.WinRate,
		NumTrades:    r.NumTrades,
		CreatedAt:    resultCreatedAt(r).Format(time.RFC3339),
		EquityCurve:  equityPointsToJSON(r.EquityCurve),
		Trades:       tradesToJSON(r.Trades),
	}
}

func resultCreatedAt(result backtest.Result) time.Time {
	if result.CreatedAt.IsZero() {
		return time.Now().UTC()
	}
	return result.CreatedAt
}

func equityPointsToJSON(points []backtest.EquityPoint) []EquityPointJSON {
	points = backtest.SampleEquityCurve(points, 2000)
	out := make([]EquityPointJSON, len(points))
	for i, point := range points {
		out[i] = EquityPointJSON(point)
	}
	return out
}

func tradesToJSON(trades []backtest.Trade) []TradeJSON {
	out := make([]TradeJSON, len(trades))
	for i, trade := range trades {
		direction := "long"
		if trade.Direction == -1 {
			direction = "short"
		}
		out[i] = TradeJSON{
			EntryTime: trade.EntryTime, ExitTime: trade.ExitTime, Direction: direction,
			EntryPrice: trade.EntryPrice, ExitPrice: trade.ExitPrice,
			NetReturn: trade.NetReturn, Profit: trade.Profit,
		}
	}
	return out
}

// ─── /api/strategies ────────────────────────────────────────────────────────

type StrategyRow struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Signals      string  `json:"signals"`
	BullSignals  string  `json:"bull_signals"`
	BearSignals  string  `json:"bear_signals"`
	Combination  string  `json:"combination"`
	RegimeFast   int     `json:"regime_fast"`
	RegimeSlow   int     `json:"regime_slow"`
	Threshold    float64 `json:"threshold"`
	FeeRate      float64 `json:"fee_rate"`
	Slippage     float64 `json:"slippage"`
	TaxRate      float64 `json:"tax_rate"`
	MinEdge      float64 `json:"min_edge"`
	RRMin        float64 `json:"rr_min"`
	MaxLoss      float64 `json:"max_loss"`
	ProfitGate   bool    `json:"profit_gate"`
	PositionSize float64 `json:"position_size"`
	Capital      float64 `json:"capital"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

const strategySelectCols = `id, name, description, signals, bull_signals, bear_signals,
	combination, regime_fast, regime_slow, threshold,
	fee_rate, slippage, tax_rate, min_edge, rr_min, max_loss, profit_gate,
	position_size, capital, created_at, updated_at`

func scanStrategy(row interface{ Scan(...any) error }) (StrategyRow, error) {
	var s StrategyRow
	var created, updated time.Time
	err := row.Scan(&s.ID, &s.Name, &s.Description, &s.Signals, &s.BullSignals, &s.BearSignals,
		&s.Combination, &s.RegimeFast, &s.RegimeSlow, &s.Threshold,
		&s.FeeRate, &s.Slippage, &s.TaxRate, &s.MinEdge, &s.RRMin,
		&s.MaxLoss, &s.ProfitGate, &s.PositionSize, &s.Capital,
		&created, &updated)
	if err != nil {
		return s, err
	}
	s.CreatedAt = created.Format(time.RFC3339)
	s.UpdatedAt = updated.Format(time.RFC3339)
	return s, nil
}

func (s *Server) handleListStrategies(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT `+strategySelectCols+` FROM strategies ORDER BY name ASC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var strategies []StrategyRow
	for rows.Next() {
		sr, err := scanStrategy(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		strategies = append(strategies, sr)
	}
	if strategies == nil {
		strategies = []StrategyRow{}
	}
	writeJSON(w, http.StatusOK, strategies)
}

func (s *Server) handleGetStrategy(w http.ResponseWriter, r *http.Request) {
	strategy, err := scanStrategy(s.db.QueryRowContext(r.Context(),
		`SELECT `+strategySelectCols+` FROM strategies WHERE id = $1`, r.PathValue("id")))
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, strategy)
}

type StrategyInput struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Signals      string  `json:"signals"`
	BullSignals  string  `json:"bull_signals"`
	BearSignals  string  `json:"bear_signals"`
	Combination  string  `json:"combination"`
	RegimeFast   int     `json:"regime_fast"`
	RegimeSlow   int     `json:"regime_slow"`
	Threshold    float64 `json:"threshold"`
	FeeRate      float64 `json:"fee_rate"`
	Slippage     float64 `json:"slippage"`
	TaxRate      float64 `json:"tax_rate"`
	MinEdge      float64 `json:"min_edge"`
	RRMin        float64 `json:"rr_min"`
	MaxLoss      float64 `json:"max_loss"`
	ProfitGate   bool    `json:"profit_gate"`
	PositionSize float64 `json:"position_size"`
	Capital      float64 `json:"capital"`
}

func defaultStrategyInput() StrategyInput {
	d := backtest.DefaultParams()
	return StrategyInput{
		Signals: strings.Join(backtest.SignalNames(), ","), Combination: string(d.Combination),
		RegimeFast: d.RegimeFast, RegimeSlow: d.RegimeSlow, Threshold: d.Threshold,
		FeeRate: d.FeeRate, Slippage: d.Slippage, TaxRate: d.TaxRate,
		MinEdge: d.MinEdge, RRMin: d.RRMin,
	}
}

func (inp StrategyInput) options() backtest.Params {
	return backtest.Params{
		Signals: splitSignals(inp.Signals), BullSignals: splitSignals(inp.BullSignals),
		BearSignals: splitSignals(inp.BearSignals), Combination: backtest.CombinationMethod(inp.Combination),
		RegimeFast: inp.RegimeFast, RegimeSlow: inp.RegimeSlow, Threshold: inp.Threshold,
		FeeRate: inp.FeeRate, Slippage: inp.Slippage, TaxRate: inp.TaxRate,
		MinEdge: inp.MinEdge, RRMin: inp.RRMin, MaxLoss: inp.MaxLoss,
		ProfitGate: inp.ProfitGate, PositionSize: inp.PositionSize, Capital: inp.Capital,
		Granularity: "1h",
	}
}

func splitSignals(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (s *Server) handleCreateStrategy(w http.ResponseWriter, r *http.Request) {
	inp := defaultStrategyInput()
	if err := json.NewDecoder(r.Body).Decode(&inp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if inp.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := inp.options().Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row := s.db.QueryRowContext(r.Context(), `
		INSERT INTO strategies (
			name, description, signals, bull_signals, bear_signals, combination,
			regime_fast, regime_slow, threshold, fee_rate, slippage, tax_rate,
			min_edge, rr_min, max_loss, profit_gate, position_size, capital
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING `+strategySelectCols,
		inp.Name, inp.Description, inp.Signals, inp.BullSignals, inp.BearSignals, inp.Combination,
		inp.RegimeFast, inp.RegimeSlow, inp.Threshold, inp.FeeRate, inp.Slippage,
		inp.TaxRate, inp.MinEdge, inp.RRMin, inp.MaxLoss, inp.ProfitGate, inp.PositionSize, inp.Capital)
	sr, err := scanStrategy(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sr)
}

func (s *Server) handleUpdateStrategy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inp := defaultStrategyInput()
	if err := json.NewDecoder(r.Body).Decode(&inp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if inp.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := inp.options().Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row := s.db.QueryRowContext(r.Context(), `
		UPDATE strategies
		SET name=$1, description=$2, signals=$3, bull_signals=$4, bear_signals=$5,
			combination=$6, regime_fast=$7, regime_slow=$8, threshold=$9,
			fee_rate=$10, slippage=$11, tax_rate=$12, min_edge=$13, rr_min=$14,
			max_loss=$15, profit_gate=$16, position_size=$17, capital=$18, updated_at=NOW()
		WHERE id=$19
		RETURNING `+strategySelectCols,
		inp.Name, inp.Description, inp.Signals, inp.BullSignals, inp.BearSignals, inp.Combination,
		inp.RegimeFast, inp.RegimeSlow, inp.Threshold, inp.FeeRate, inp.Slippage,
		inp.TaxRate, inp.MinEdge, inp.RRMin, inp.MaxLoss, inp.ProfitGate, inp.PositionSize, inp.Capital, id)
	sr, err := scanStrategy(row)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sr)
}

func (s *Server) handleDeleteStrategy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM strategies WHERE id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type RunStrategyRequest struct {
	Exchange    string `json:"exchange"`
	ProductID   string `json:"product_id"`
	Granularity string `json:"granularity"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Save        bool   `json:"save"`
}

func (s *Server) handleRunStrategy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d := backtest.DefaultParams()
	req := RunStrategyRequest{Exchange: d.Exchange, Granularity: d.Granularity}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ProductID == "" {
		writeError(w, http.StatusBadRequest, "product_id required")
		return
	}

	stRow := s.db.QueryRowContext(r.Context(),
		`SELECT `+strategySelectCols+` FROM strategies WHERE id=$1`, id)
	st, err := scanStrategy(stRow)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "strategy not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	start, err := parseDate(req.Start)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	end, err := parseDate(req.End)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if start.IsZero() {
		start = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}

	params := backtest.Params{
		Exchange:     req.Exchange,
		ProductID:    req.ProductID,
		Granularity:  req.Granularity,
		Start:        start,
		End:          end,
		Signals:      splitSignals(st.Signals),
		BullSignals:  splitSignals(st.BullSignals),
		BearSignals:  splitSignals(st.BearSignals),
		Combination:  backtest.CombinationMethod(st.Combination),
		RegimeFast:   st.RegimeFast,
		RegimeSlow:   st.RegimeSlow,
		Threshold:    st.Threshold,
		FeeRate:      st.FeeRate,
		Slippage:     st.Slippage,
		TaxRate:      st.TaxRate,
		MinEdge:      st.MinEdge,
		RRMin:        st.RRMin,
		MaxLoss:      st.MaxLoss,
		ProfitGate:   st.ProfitGate,
		PositionSize: st.PositionSize,
		Capital:      st.Capital,
	}
	strategyID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid strategy id")
		return
	}
	result, err := s.backtests.Run(r.Context(), backtest.RunRequest{Params: params, Save: req.Save, StrategyID: &strategyID})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, backtestResultToRow(result))
}
