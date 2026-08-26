package backtest

import (
	"math"
	"testing"
	"time"
)

func testCandles(n int) []Candle {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]Candle, n)
	for i := range candles {
		// Alternating changes provide non-zero rolling volatility for the entry gate.
		price := 100 + float64(i)
		if i%2 == 0 {
			price += 2
		}
		candles[i] = Candle{
			Time: start.Add(time.Duration(i) * time.Hour),
			Open: price, High: price, Low: price, Close: price, Volume: 1,
		}
	}
	return candles
}

func scriptedSignal(values map[int]SignalValue) SignalFunc {
	return func(candles []Candle) []SignalValue {
		out := make([]SignalValue, len(candles))
		for index, value := range values {
			out[index] = value
		}
		return out
	}
}

func scriptedParams() Params {
	p := DefaultParams()
	p.Signals = []string{"scripted"}
	p.FeeRate = 0
	p.Slippage = 0
	p.TaxRate = 0
	p.MinEdge = 0
	p.RRMin = 0
	return p
}

func runScripted(t *testing.T, candles []Candle, p Params, values map[int]SignalValue) Result {
	t.Helper()
	result, err := runWithSignalSet(candles, p, map[string]SignalFunc{
		"scripted": scriptedSignal(values),
	})
	if err != nil {
		t.Fatalf("run backtest: %v", err)
	}
	return result
}

func TestCombineSignals(t *testing.T) {
	signals := [][]SignalValue{
		{Buy, Buy, Sell, Buy},
		{Buy, Neutral, Sell, Sell},
		{Neutral, Sell, Sell, Buy},
	}

	voting := combineSignals(signals, Voting, 0.3)
	wantVoting := []SignalValue{Buy, Neutral, Sell, Buy}
	assertSignals(t, voting, wantVoting)

	consensus := combineSignals(signals, Consensus, 0.5)
	wantConsensus := []SignalValue{Buy, Neutral, Sell, Buy}
	assertSignals(t, consensus, wantConsensus)

	weighted := combineSignals(signals, Weighted, 0.3)
	assertSignals(t, weighted, voting)
}

func TestRunModelsLongShortCostsAndTax(t *testing.T) {
	candles := testCandles(40)
	candles[12].Close = 100
	candles[13].Close = 110
	candles[14].Close = 100
	candles[15].Close = 90
	p := scriptedParams()
	p.FeeRate = 0.01
	p.Slippage = 0.005
	p.TaxRate = 0.20

	result := runScripted(t, candles, p, map[int]SignalValue{
		12: Buy,
		13: Neutral,
		14: Sell,
		15: Neutral,
	})

	if result.NumTrades != 2 {
		t.Fatalf("trades = %d, want 2", result.NumTrades)
	}
	if result.Trades[0].Direction != 1 || result.Trades[1].Direction != -1 {
		t.Fatalf("directions = %d,%d, want long,short", result.Trades[0].Direction, result.Trades[1].Direction)
	}
	// Each trade returns 10%% - (2*1%% fee + .5%% slippage) = 7.5%%.
	wantBeforeTax := (1.075 * 1.075) - 1
	want := wantBeforeTax * 0.8
	assertClose(t, result.TotalReturn, want, 1e-12)
	assertClose(t, result.WinRate, 1, 1e-12)
}

func TestRunRiskGates(t *testing.T) {
	t.Run("minimum edge rejects entry", func(t *testing.T) {
		candles := testCandles(40)
		p := scriptedParams()
		p.MinEdge = 0.50
		result := runScripted(t, candles, p, map[int]SignalValue{12: Buy, 13: Neutral})
		if result.NumTrades != 0 {
			t.Fatalf("trades = %d, want 0", result.NumTrades)
		}
	})

	t.Run("maximum loss forces close", func(t *testing.T) {
		candles := testCandles(40)
		candles[12].Close = 100
		candles[13].Close = 90
		p := scriptedParams()
		p.MaxLoss = 0.05
		result := runScripted(t, candles, p, map[int]SignalValue{12: Buy})
		if result.NumTrades != 1 {
			t.Fatalf("trades = %d, want 1", result.NumTrades)
		}
		assertClose(t, result.Trades[0].NetReturn, -0.10, 1e-12)
	})

	t.Run("profit gate holds until profitable", func(t *testing.T) {
		candles := testCandles(40)
		candles[12].Close = 100
		candles[13].Close = 90
		candles[14].Close = 110
		p := scriptedParams()
		p.ProfitGate = true
		result := runScripted(t, candles, p, map[int]SignalValue{12: Buy})
		if result.NumTrades != 1 || !result.Trades[0].ExitTime.Equal(candles[14].Time) {
			t.Fatalf("trade = %+v, want profitable exit at candle 14", result.Trades)
		}
		assertClose(t, result.TotalReturn, 0.10, 1e-12)
	})
}

func TestRunFixedPositionSizing(t *testing.T) {
	candles := testCandles(40)
	candles[12].Close = 100
	candles[13].Close = 110
	p := scriptedParams()
	p.PositionSize = 1_000
	p.Capital = 10_000

	result := runScripted(t, candles, p, map[int]SignalValue{12: Buy, 13: Neutral})
	assertClose(t, result.TotalReturn, 0.01, 1e-12)
	if len(result.EquityCurve) != len(candles) {
		t.Fatalf("equity points = %d, want %d", len(result.EquityCurve), len(candles))
	}
	assertClose(t, result.EquityCurve[len(result.EquityCurve)-1].Equity, 10_100, 1e-9)
	if math.IsNaN(result.Sharpe) || math.IsInf(result.Sharpe, 0) ||
		math.IsNaN(result.Sortino) || math.IsInf(result.Sortino, 0) ||
		math.IsNaN(result.Calmar) || math.IsInf(result.Calmar, 0) {
		t.Fatalf("metrics must be finite: %+v", result)
	}
}

func TestSampleEquityCurvePreservesEndpoints(t *testing.T) {
	points := make([]EquityPoint, 10)
	for i := range points {
		points[i] = EquityPoint{Equity: float64(i)}
	}
	sampled := SampleEquityCurve(points, 4)
	if len(sampled) != 4 || sampled[0].Equity != 0 || sampled[len(sampled)-1].Equity != 9 {
		t.Fatalf("sampled curve = %+v", sampled)
	}
	if got := len(SampleEquityCurve(points, 20)); got != len(points) {
		t.Fatalf("unsampled length = %d, want %d", got, len(points))
	}
}

func TestRunValidatesConfiguration(t *testing.T) {
	candles := testCandles(40)
	cases := []struct {
		name   string
		mutate func(*Params)
	}{
		{"granularity", func(p *Params) { p.Granularity = "3h" }},
		{"combination", func(p *Params) { p.Combination = "magic" }},
		{"threshold", func(p *Params) { p.Threshold = 2 }},
		{"adaptive signals", func(p *Params) { p.Combination = Adaptive }},
		{"position exceeds capital", func(p *Params) { p.PositionSize, p.Capital = 200, 100 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultParams()
			p.Signals = []string{"rsi"}
			tc.mutate(&p)
			if _, err := Run(candles, p); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	p := DefaultParams()
	p.Signals = []string{"unknown"}
	if _, err := Run(candles, p); err == nil {
		t.Fatal("expected unknown signal error")
	}
	if _, err := Run(candles[:20], DefaultParams()); err == nil {
		t.Fatal("expected insufficient candle error")
	}
}

func TestDetectRegimeAndSignalShapes(t *testing.T) {
	candles := testCandles(250)
	for i := range candles {
		candles[i].Close = 100 + float64(i)
	}
	regimes := DetectRegime(candles, 10, 30)
	if regimes[len(regimes)-1] != RegimeBull {
		t.Fatalf("last regime = %v, want bull", regimes[len(regimes)-1])
	}
	for name, signal := range AllSignals() {
		if got := len(signal(candles)); got != len(candles) {
			t.Fatalf("%s signal length = %d, want %d", name, got, len(candles))
		}
	}
}

func TestRunCombinationsRanksAndLimits(t *testing.T) {
	candles := testCandles(250)
	p := DefaultParams()
	p.FeeRate, p.Slippage, p.TaxRate, p.MinEdge, p.RRMin = 0, 0, 0, 0, 0
	results, err := RunCombinations(candles, p, 3)
	if err != nil {
		t.Fatalf("run combinations: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for i := 1; i < len(results); i++ {
		if results[i].Sharpe > results[i-1].Sharpe {
			t.Fatalf("results are not sorted by Sharpe: %v > %v", results[i].Sharpe, results[i-1].Sharpe)
		}
	}
}

func assertSignals(t *testing.T, got, want []SignalValue) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("signal length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("signal[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func assertClose(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("got %.12f, want %.12f (tolerance %.12f)", got, want, tolerance)
	}
}
