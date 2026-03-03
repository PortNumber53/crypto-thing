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

	_ "github.com/lib/pq"
)

// Server is the HTTP API server.
type Server struct {
	db  *sql.DB
	cfg *config.Config
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
	return &Server{db: db, cfg: cfg}, nil
}

// Close releases the DB connection pool.
func (s *Server) Close() { s.db.Close() }

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
	mux.HandleFunc("DELETE /api/backtest/results/{id}", s.handleDeleteBacktest)

	mux.HandleFunc("GET /api/strategies", s.handleListStrategies)
	mux.HandleFunc("POST /api/strategies", s.handleCreateStrategy)
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
	Products   int   `json:"products"`
	Candles    int64 `json:"candles"`
	Backtests  int   `json:"backtests"`
	Strategies int   `json:"strategies"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	var stats StatsResponse
	row := s.db.QueryRowContext(r.Context(), `
		SELECT
			(SELECT COUNT(*) FROM products) AS products,
			(SELECT COUNT(*) FROM candles WHERE volume >= 0) AS candles,
			(SELECT COUNT(*) FROM backtest_results) AS backtests,
			(SELECT COUNT(*) FROM strategies) AS strategies
	`)
	if err := row.Scan(&stats.Products, &stats.Candles, &stats.Backtests, &stats.Strategies); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
			LEFT JOIN candles c ON c.exchange = p.exchange AND c.product_id = p.product_id
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
			LEFT JOIN candles c ON c.exchange = p.exchange AND c.product_id = p.product_id
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

	granSec := granularitySecs(granularity)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT time, open, high, low, close, volume
		FROM candles
		WHERE exchange = $1 AND product_id = $2
		  AND time >= $3 AND time < $4
		  AND volume >= 0
		  AND EXTRACT(EPOCH FROM time)::bigint % $5 = 0
		ORDER BY time ASC
		LIMIT $6
	`, exchange, product, start, end, granSec, limit)
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

func granularitySecs(g string) int64 {
	switch strings.ToLower(g) {
	case "1m":
		return 60
	case "5m":
		return 300
	case "15m":
		return 900
	case "30m":
		return 1800
	case "1h":
		return 3600
	case "2h":
		return 7200
	case "6h":
		return 21600
	case "1d":
		return 86400
	default:
		return 3600
	}
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
	writeJSON(w, http.StatusOK, signals)
}

// ─── /api/backtest ──────────────────────────────────────────────────────────

type BacktestResultRow struct {
	ID          int64   `json:"id"`
	Exchange    string  `json:"exchange"`
	ProductID   string  `json:"product_id"`
	Granularity string  `json:"granularity"`
	StartTime   string  `json:"start_time"`
	EndTime     string  `json:"end_time"`
	Signals     string  `json:"signals"`
	Combination string  `json:"combination"`
	Threshold   float64 `json:"threshold"`
	FeeRate     float64 `json:"fee_rate"`
	Slippage    float64 `json:"slippage"`
	TaxRate     float64 `json:"tax_rate"`
	MinEdge     float64 `json:"min_edge"`
	RRMin       float64 `json:"rr_min"`
	TotalReturn float64 `json:"total_return"`
	Sharpe      float64 `json:"sharpe"`
	Sortino     float64 `json:"sortino"`
	MaxDrawdown float64 `json:"max_drawdown"`
	Calmar      float64 `json:"calmar"`
	WinRate     float64 `json:"win_rate"`
	NumTrades   int     `json:"num_trades"`
	CreatedAt   string  `json:"created_at"`
}

func scanBacktestRow(rows interface {
	Scan(...any) error
}) (BacktestResultRow, error) {
	var r BacktestResultRow
	var start, end, created time.Time
	err := rows.Scan(
		&r.ID, &r.Exchange, &r.ProductID, &r.Granularity,
		&start, &end,
		&r.Signals, &r.Combination, &r.Threshold,
		&r.FeeRate, &r.Slippage, &r.TaxRate, &r.MinEdge, &r.RRMin,
		&r.TotalReturn, &r.Sharpe, &r.Sortino, &r.MaxDrawdown,
		&r.Calmar, &r.WinRate, &r.NumTrades, &created,
	)
	if err != nil {
		return r, err
	}
	r.StartTime = start.Format(time.RFC3339)
	r.EndTime = end.Format(time.RFC3339)
	r.CreatedAt = created.Format(time.RFC3339)
	return r, nil
}

const backtestSelectCols = `
	id, exchange, product_id, granularity, start_time, end_time,
	signals, combination, threshold, fee_rate, slippage, tax_rate, min_edge, rr_min,
	total_return, sharpe, sortino, max_drawdown, calmar, win_rate, num_trades, created_at`

func (s *Server) handleListBacktests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	product := q.Get("product")
	limit := 50
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	var rows *sql.Rows
	var err error
	if product != "" {
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
	Exchange    string   `json:"exchange"`
	ProductID   string   `json:"product_id"`
	Granularity string   `json:"granularity"`
	Start       string   `json:"start"`
	End         string   `json:"end"`
	Signals     []string `json:"signals"`
	Combination string   `json:"combination"`
	Threshold   float64  `json:"threshold"`
	FeeRate     float64  `json:"fee_rate"`
	Slippage    float64  `json:"slippage"`
	TaxRate     float64  `json:"tax_rate"`
	MinEdge     float64  `json:"min_edge"`
	RRMin       float64  `json:"rr_min"`
	Save        bool     `json:"save"`
}

func (s *Server) handleRunBacktest(w http.ResponseWriter, r *http.Request) {
	var req RunBacktestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if req.ProductID == "" {
		writeError(w, http.StatusBadRequest, "product_id required")
		return
	}
	if req.Exchange == "" {
		req.Exchange = "coinbase"
	}
	if req.Granularity == "" {
		req.Granularity = "1h"
	}
	start, _ := parseDate(req.Start)
	end, _ := parseDate(req.End)
	if start.IsZero() {
		start = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}

	candles, err := backtest.LoadCandles(r.Context(), s.cfg.Database.URL, req.Exchange, req.ProductID, start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load candles: "+err.Error())
		return
	}

	params := backtest.Params{
		Exchange:    req.Exchange,
		ProductID:   req.ProductID,
		Granularity: req.Granularity,
		Start:       start,
		End:         end,
		Signals:     req.Signals,
		Combination: backtest.CombinationMethod(req.Combination),
		Threshold:   req.Threshold,
		FeeRate:     req.FeeRate,
		Slippage:    req.Slippage,
		TaxRate:     req.TaxRate,
		MinEdge:     req.MinEdge,
		RRMin:       req.RRMin,
	}

	result, err := backtest.Run(candles, params)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if req.Save {
		_ = backtest.SaveResult(r.Context(), s.cfg.Database.URL, result)
	}

	writeJSON(w, http.StatusOK, backtestResultToRow(result))
}

func backtestResultToRow(r backtest.Result) BacktestResultRow {
	return BacktestResultRow{
		Exchange:    r.Exchange,
		ProductID:   r.ProductID,
		Granularity: r.Granularity,
		StartTime:   r.Start.Format(time.RFC3339),
		EndTime:     r.End.Format(time.RFC3339),
		Signals:     r.Signals,
		Combination: r.Combination,
		Threshold:   r.Threshold,
		FeeRate:     r.FeeRate,
		Slippage:    r.Slippage,
		TaxRate:     r.TaxRate,
		MinEdge:     r.MinEdge,
		RRMin:       r.RRMin,
		TotalReturn: r.TotalReturn,
		Sharpe:      r.Sharpe,
		Sortino:     r.Sortino,
		MaxDrawdown: r.MaxDrawdown,
		Calmar:      r.Calmar,
		WinRate:     r.WinRate,
		NumTrades:   r.NumTrades,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
}

// ─── /api/strategies ────────────────────────────────────────────────────────

type StrategyRow struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Signals     string  `json:"signals"`
	Combination string  `json:"combination"`
	Threshold   float64 `json:"threshold"`
	FeeRate     float64 `json:"fee_rate"`
	Slippage    float64 `json:"slippage"`
	TaxRate     float64 `json:"tax_rate"`
	MinEdge     float64 `json:"min_edge"`
	RRMin       float64 `json:"rr_min"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

const strategySelectCols = `id, name, description, signals, combination, threshold,
	fee_rate, slippage, tax_rate, min_edge, rr_min, created_at, updated_at`

func scanStrategy(row interface{ Scan(...any) error }) (StrategyRow, error) {
	var s StrategyRow
	var created, updated time.Time
	err := row.Scan(&s.ID, &s.Name, &s.Description, &s.Signals, &s.Combination,
		&s.Threshold, &s.FeeRate, &s.Slippage, &s.TaxRate, &s.MinEdge, &s.RRMin,
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

type StrategyInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Signals     string  `json:"signals"`
	Combination string  `json:"combination"`
	Threshold   float64 `json:"threshold"`
	FeeRate     float64 `json:"fee_rate"`
	Slippage    float64 `json:"slippage"`
	TaxRate     float64 `json:"tax_rate"`
	MinEdge     float64 `json:"min_edge"`
	RRMin       float64 `json:"rr_min"`
}

func (s *Server) handleCreateStrategy(w http.ResponseWriter, r *http.Request) {
	var inp StrategyInput
	if err := json.NewDecoder(r.Body).Decode(&inp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if inp.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	setDefaults(&inp)
	row := s.db.QueryRowContext(r.Context(), `
		INSERT INTO strategies (name, description, signals, combination, threshold, fee_rate, slippage, tax_rate, min_edge, rr_min)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+strategySelectCols,
		inp.Name, inp.Description, inp.Signals, inp.Combination,
		inp.Threshold, inp.FeeRate, inp.Slippage, inp.TaxRate, inp.MinEdge, inp.RRMin)
	sr, err := scanStrategy(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sr)
}

func (s *Server) handleUpdateStrategy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var inp StrategyInput
	if err := json.NewDecoder(r.Body).Decode(&inp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	setDefaults(&inp)
	row := s.db.QueryRowContext(r.Context(), `
		UPDATE strategies
		SET name=$1, description=$2, signals=$3, combination=$4, threshold=$5,
			fee_rate=$6, slippage=$7, tax_rate=$8, min_edge=$9, rr_min=$10, updated_at=NOW()
		WHERE id=$11
		RETURNING `+strategySelectCols,
		inp.Name, inp.Description, inp.Signals, inp.Combination,
		inp.Threshold, inp.FeeRate, inp.Slippage, inp.TaxRate, inp.MinEdge, inp.RRMin, id)
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
	var req RunStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ProductID == "" {
		writeError(w, http.StatusBadRequest, "product_id required")
		return
	}
	if req.Exchange == "" {
		req.Exchange = "coinbase"
	}
	if req.Granularity == "" {
		req.Granularity = "1h"
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

	start, _ := parseDate(req.Start)
	end, _ := parseDate(req.End)
	if start.IsZero() {
		start = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}

	candles, err := backtest.LoadCandles(r.Context(), s.cfg.Database.URL, req.Exchange, req.ProductID, start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load candles: "+err.Error())
		return
	}

	signals := strings.Split(st.Signals, ",")
	params := backtest.Params{
		Exchange:    req.Exchange,
		ProductID:   req.ProductID,
		Granularity: req.Granularity,
		Start:       start,
		End:         end,
		Signals:     signals,
		Combination: backtest.CombinationMethod(st.Combination),
		Threshold:   st.Threshold,
		FeeRate:     st.FeeRate,
		Slippage:    st.Slippage,
		TaxRate:     st.TaxRate,
		MinEdge:     st.MinEdge,
		RRMin:       st.RRMin,
	}

	result, err := backtest.Run(candles, params)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if req.Save {
		_ = backtest.SaveResult(r.Context(), s.cfg.Database.URL, result)
	}

	writeJSON(w, http.StatusOK, backtestResultToRow(result))
}

func setDefaults(inp *StrategyInput) {
	if inp.Signals == "" {
		inp.Signals = "rsi,macd,bbands,ema_cross"
	}
	if inp.Combination == "" {
		inp.Combination = "voting"
	}
	if inp.Threshold == 0 {
		inp.Threshold = 0.5
	}
	if inp.FeeRate == 0 {
		inp.FeeRate = 0.001
	}
	if inp.Slippage == 0 {
		inp.Slippage = 0.001
	}
	if inp.TaxRate == 0 {
		inp.TaxRate = 0.30
	}
	if inp.MinEdge == 0 {
		inp.MinEdge = 0.005
	}
	if inp.RRMin == 0 {
		inp.RRMin = 1.5
	}
}
