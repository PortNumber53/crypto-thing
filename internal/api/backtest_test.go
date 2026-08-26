package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cryptool/internal/backtest"
)

type fakeBacktestRunner struct {
	runRequest   backtest.RunRequest
	comboRequest backtest.CombinationRequest
	result       backtest.Result
	results      []backtest.Result
	err          error
}

func (f *fakeBacktestRunner) Run(_ context.Context, request backtest.RunRequest) (backtest.Result, error) {
	f.runRequest = request
	return f.result, f.err
}

func (f *fakeBacktestRunner) RunCombinations(_ context.Context, request backtest.CombinationRequest) ([]backtest.Result, error) {
	f.comboRequest = request
	return f.results, f.err
}

func TestRunBacktestRESTDefaultsAndDetails(t *testing.T) {
	created := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &fakeBacktestRunner{result: backtest.Result{
		ID: 42, Exchange: "coinbase", ProductID: "BTC-USD", Granularity: "1h",
		Start: created.Add(-time.Hour), End: created, Signals: "rsi", Combination: "voting",
		Threshold: 0.5, CreatedAt: created,
		EquityCurve: []backtest.EquityPoint{{Time: created, Equity: 101, Price: 100}},
		Trades:      []backtest.Trade{{EntryTime: created.Add(-time.Hour), ExitTime: created, Direction: -1, Profit: true}},
	}}
	server := &Server{backtests: fake}

	recorder := requestJSON(t, server.Handler(), http.MethodPost, "/api/backtest/run", `{"product_id":"BTC-USD"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	params := fake.runRequest.Params
	if params.Exchange != "coinbase" || params.Granularity != "1h" || params.FeeRate != 0.001 ||
		params.Slippage != 0.001 || params.TaxRate != 0.30 || params.RegimeFast != 50 || params.RegimeSlow != 200 {
		t.Fatalf("defaults not forwarded: %+v", params)
	}
	var response BacktestResultRow
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 42 || len(response.EquityCurve) != 1 || len(response.Trades) != 1 || response.Trades[0].Direction != "short" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestRunBacktestRESTForwardsAdvancedOptionsAndExplicitZeros(t *testing.T) {
	fake := &fakeBacktestRunner{}
	server := &Server{backtests: fake}
	body := `{
		"product_id":"ETH-USD","granularity":"5m","start":"2024-01-01","end":"2024-02-01",
		"signals":["sma"],"bull_signals":["sma","ema_cross"],"bear_signals":["rsi"],
		"combination":"adaptive","regime_fast":10,"regime_slow":30,"threshold":0.7,
		"fee_rate":0,"slippage":0,"tax_rate":0,"min_edge":0,"rr_min":0,
		"max_loss":0.04,"profit_gate":true,"position_size":500,"capital":5000,"save":true
	}`
	recorder := requestJSON(t, server.Handler(), http.MethodPost, "/api/backtest/run", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	p := fake.runRequest.Params
	if p.Combination != backtest.Adaptive || p.RegimeFast != 10 || p.RegimeSlow != 30 ||
		p.FeeRate != 0 || p.Slippage != 0 || p.TaxRate != 0 || p.RRMin != 0 ||
		p.MaxLoss != 0.04 || !p.ProfitGate || p.PositionSize != 500 || p.Capital != 5000 || !fake.runRequest.Save {
		t.Fatalf("advanced options not forwarded: %+v", fake.runRequest)
	}
}

func TestRunBacktestRESTValidation(t *testing.T) {
	fake := &fakeBacktestRunner{}
	server := &Server{backtests: fake}
	cases := []string{
		`{}`,
		`{"product_id":"BTC-USD","start":"bad-date"}`,
		`{"product_id":"BTC-USD","combination":"adaptive"}`,
		`{"product_id":"BTC-USD","position_size":200,"capital":100}`,
	}
	for _, body := range cases {
		recorder := requestJSON(t, server.Handler(), http.MethodPost, "/api/backtest/run", body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, response = %s", body, recorder.Code, recorder.Body.String())
		}
	}
}

func TestRunBacktestRESTServiceError(t *testing.T) {
	fake := &fakeBacktestRunner{err: errors.New("engine failed")}
	server := &Server{backtests: fake}
	recorder := requestJSON(t, server.Handler(), http.MethodPost, "/api/backtest/run", `{"product_id":"BTC-USD"}`)
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), "engine failed") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRunBacktestCombinationsREST(t *testing.T) {
	fake := &fakeBacktestRunner{results: []backtest.Result{{Signals: "rsi", EquityCurve: []backtest.EquityPoint{{Equity: 1}}}}}
	server := &Server{backtests: fake}
	recorder := requestJSON(t, server.Handler(), http.MethodPost, "/api/backtest/combo", `{"product_id":"BTC-USD","top":3,"save":true}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if fake.comboRequest.Top != 3 || !fake.comboRequest.Save {
		t.Fatalf("combo request = %+v", fake.comboRequest)
	}
	if strings.Contains(recorder.Body.String(), `"equity_curve":`) || strings.Contains(recorder.Body.String(), `"trades":`) {
		t.Fatalf("combination response should contain summaries only: %s", recorder.Body.String())
	}

	recorder = requestJSON(t, server.Handler(), http.MethodPost, "/api/backtest/combo", `{"product_id":"BTC-USD","top":32}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid top status = %d", recorder.Code)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
