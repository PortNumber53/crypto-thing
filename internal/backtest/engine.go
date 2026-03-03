package backtest

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// CombinationMethod controls how multiple signals are merged.
type CombinationMethod string

const (
	Voting    CombinationMethod = "voting"
	Consensus CombinationMethod = "consensus"
	Weighted  CombinationMethod = "weighted"
)

// Params holds all backtest configuration parameters.
type Params struct {
	Exchange    string
	ProductID   string
	Granularity string
	Start       time.Time
	End         time.Time

	Signals     []string
	Combination CombinationMethod
	Threshold   float64 // voting threshold [0,1]; default 0.5

	FeeRate  float64 // round-trip fee per leg, e.g. 0.001 = 0.1%
	Slippage float64 // estimated slippage per trade, e.g. 0.001
	TaxRate  float64 // fraction of profits taxed, e.g. 0.30
	MinEdge  float64 // min expected net gain to enter a trade, e.g. 0.005
	RRMin    float64 // minimum risk/reward ratio, e.g. 1.5
}

// Trade records a single completed trade.
type Trade struct {
	EntryTime  time.Time
	ExitTime   time.Time
	EntryPrice float64
	ExitPrice  float64
	Direction  int // +1 long, -1 short
	NetReturn  float64
	Profit     bool
}

// Result holds the final performance metrics for a backtest run.
type Result struct {
	Exchange    string
	ProductID   string
	Granularity string
	Start       time.Time
	End         time.Time
	Signals     string
	Combination string
	Threshold   float64
	FeeRate     float64
	Slippage    float64
	TaxRate     float64
	MinEdge     float64
	RRMin       float64

	TotalReturn float64
	Sharpe      float64
	Sortino     float64
	MaxDrawdown float64
	Calmar      float64
	WinRate     float64
	NumTrades   int
	Trades      []Trade
}

// position tracks an open trade.
type position struct {
	dir        int // +1 long, -1 short
	entryPrice float64
	entryTime  time.Time
}

// Run executes a backtest and returns the Result.
func Run(candles []Candle, p Params) (Result, error) {
	if len(candles) < 30 {
		return Result{}, fmt.Errorf("need at least 30 candles, got %d", len(candles))
	}

	// Build signal map
	available := AllSignals()
	var sigFuncs []SignalFunc
	for _, name := range p.Signals {
		fn, ok := available[name]
		if !ok {
			return Result{}, fmt.Errorf("unknown signal %q; available: %s", name, availableSignalNames())
		}
		sigFuncs = append(sigFuncs, fn)
	}
	if len(sigFuncs) == 0 {
		// default: all
		for _, fn := range available {
			sigFuncs = append(sigFuncs, fn)
		}
	}

	// Compute individual signal series
	allSigs := make([][]SignalValue, len(sigFuncs))
	for i, fn := range sigFuncs {
		allSigs[i] = fn(candles)
	}

	// Combine signals
	threshold := p.Threshold
	if threshold <= 0 {
		threshold = 0.5
	}
	combined := combineSignals(allSigs, p.Combination, threshold)

	// Determine granularity in seconds for expected-move window
	granSec := granularityToSeconds(p.Granularity)

	// Rolling volatility window for profitability gate (last 10 buckets)
	const volWindow = 10

	// Simulate trades
	var (
		pos       *position
		portfolio = []float64{1.0} // normalized
		returns   []float64
		trades    []Trade
	)

	roundTripCost := p.FeeRate*2 + p.Slippage

	for i := 1; i < len(candles); i++ {
		sig := combined[i]
		price := candles[i].Close

		// --- Profitability gate ---
		// Estimate expected move as rolling stddev of returns
		if sig != Neutral && pos == nil {
			expectedMove := rollingVolatility(candles, i, volWindow)
			// Stop-loss proxy: half the expected move; take-profit: full expected move
			tp := price * (1 + expectedMove)
			sl := price * (1 - expectedMove/2)
			rr := 0.0
			if price-sl > 0 {
				rr = (tp - price) / (price - sl)
			}
			netGain := expectedMove - roundTripCost

			// Require positive edge AND minimum RR ratio
			rrMin := p.RRMin
			if rrMin <= 0 {
				rrMin = 1.5
			}
			if netGain <= p.MinEdge || (p.RRMin > 0 && rr < rrMin) {
				sig = Neutral
			}
		}
		_ = granSec

		// --- Position management ---
		switch {
		case sig == Buy && pos == nil:
			// Enter long
			pos = &position{dir: +1, entryPrice: price, entryTime: candles[i].Time}

		case sig == Sell && pos == nil:
			// Enter short
			pos = &position{dir: -1, entryPrice: price, entryTime: candles[i].Time}

		case sig == Sell && pos != nil && pos.dir == +1:
			// Close long, enter short
			t := closeTrade(pos, price, candles[i].Time, p.FeeRate, p.Slippage)
			trades = append(trades, t)
			r := t.NetReturn
			portfolio = append(portfolio, portfolio[len(portfolio)-1]*(1+r))
			returns = append(returns, r)
			pos = &position{dir: -1, entryPrice: price, entryTime: candles[i].Time}

		case sig == Buy && pos != nil && pos.dir == -1:
			// Close short, enter long
			t := closeTrade(pos, price, candles[i].Time, p.FeeRate, p.Slippage)
			trades = append(trades, t)
			r := t.NetReturn
			portfolio = append(portfolio, portfolio[len(portfolio)-1]*(1+r))
			returns = append(returns, r)
			pos = &position{dir: +1, entryPrice: price, entryTime: candles[i].Time}

		case sig == Neutral && pos != nil:
			// Exit to flat
			t := closeTrade(pos, price, candles[i].Time, p.FeeRate, p.Slippage)
			trades = append(trades, t)
			r := t.NetReturn
			portfolio = append(portfolio, portfolio[len(portfolio)-1]*(1+r))
			returns = append(returns, r)
			pos = nil

		default:
			portfolio = append(portfolio, portfolio[len(portfolio)-1])
		}
	}

	// Close any open position at last price
	if pos != nil {
		last := candles[len(candles)-1]
		t := closeTrade(pos, last.Close, last.Time, p.FeeRate, p.Slippage)
		trades = append(trades, t)
		r := t.NetReturn
		portfolio = append(portfolio, portfolio[len(portfolio)-1]*(1+r))
		returns = append(returns, r)
		pos = nil
	}
	_ = pos

	// Apply tax on net gain
	finalValue := portfolio[len(portfolio)-1]
	totalReturn := finalValue - 1.0
	if totalReturn > 0 && p.TaxRate > 0 {
		totalReturn = totalReturn * (1 - p.TaxRate)
		finalValue = 1 + totalReturn
	}

	// Metrics
	sharpe := calcSharpe(returns, p.Granularity)
	sortino := calcSortino(returns, p.Granularity)
	maxDD := calcMaxDrawdown(portfolio)
	calmar := 0.0
	annReturn := annualizedReturn(totalReturn, len(candles), granularityToSeconds(p.Granularity))
	if maxDD != 0 {
		calmar = annReturn / math.Abs(maxDD)
	}

	winRate := 0.0
	if len(trades) > 0 {
		wins := 0
		for _, t := range trades {
			if t.Profit {
				wins++
			}
		}
		winRate = float64(wins) / float64(len(trades))
	}

	return Result{
		Exchange:    p.Exchange,
		ProductID:   p.ProductID,
		Granularity: p.Granularity,
		Start:       p.Start,
		End:         p.End,
		Signals:     strings.Join(p.Signals, ","),
		Combination: string(p.Combination),
		Threshold:   p.Threshold,
		FeeRate:     p.FeeRate,
		Slippage:    p.Slippage,
		TaxRate:     p.TaxRate,
		MinEdge:     p.MinEdge,
		RRMin:       p.RRMin,
		TotalReturn: totalReturn,
		Sharpe:      sharpe,
		Sortino:     sortino,
		MaxDrawdown: maxDD,
		Calmar:      calmar,
		WinRate:     winRate,
		NumTrades:   len(trades),
		Trades:      trades,
	}, nil
}

// combineSignals merges multiple signal series into one using the given method.
func combineSignals(allSigs [][]SignalValue, method CombinationMethod, threshold float64) []SignalValue {
	if len(allSigs) == 0 {
		return nil
	}
	n := len(allSigs[0])
	out := make([]SignalValue, n)
	k := len(allSigs)

	for i := 0; i < n; i++ {
		switch method {
		case Consensus:
			buyCount, sellCount := 0, 0
			for _, s := range allSigs {
				if s[i] == Buy {
					buyCount++
				} else if s[i] == Sell {
					sellCount++
				}
			}
			majority := k/2 + 1
			if buyCount >= majority {
				out[i] = Buy
			} else if sellCount >= majority {
				out[i] = Sell
			}

		case Weighted:
			// Equal weights — same as voting average
			fallthrough
		default: // Voting
			var sum float64
			for _, s := range allSigs {
				sum += float64(s[i])
			}
			avg := sum / float64(k)
			switch {
			case avg > threshold:
				out[i] = Buy
			case avg < -threshold:
				out[i] = Sell
			default:
				out[i] = Neutral
			}
		}
	}
	return out
}

// closeTrade computes the net return for closing a position.
func closeTrade(pos *position, exitPrice float64, exitTime time.Time, feeRate, slippage float64) Trade {
	roundTrip := feeRate*2 + slippage
	var raw float64
	if pos.dir == +1 {
		raw = (exitPrice - pos.entryPrice) / pos.entryPrice
	} else {
		raw = (pos.entryPrice - exitPrice) / pos.entryPrice
	}
	net := raw - roundTrip
	return Trade{
		EntryTime:  pos.entryTime,
		ExitTime:   exitTime,
		EntryPrice: pos.entryPrice,
		ExitPrice:  exitPrice,
		Direction:  pos.dir,
		NetReturn:  net,
		Profit:     net > 0,
	}
}

// rollingVolatility returns the stddev of returns over the last `window` candles ending at index i.
func rollingVolatility(candles []Candle, i, window int) float64 {
	start := i - window
	if start < 1 {
		start = 1
	}
	returns := make([]float64, 0, window)
	for j := start; j <= i; j++ {
		if candles[j-1].Close != 0 {
			returns = append(returns, (candles[j].Close-candles[j-1].Close)/candles[j-1].Close)
		}
	}
	if len(returns) == 0 {
		return 0
	}
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))
	var variance float64
	for _, r := range returns {
		diff := r - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(returns)))
}

// calcSharpe computes the annualized Sharpe ratio (assuming 0% risk-free rate).
func calcSharpe(returns []float64, granularity string) float64 {
	if len(returns) < 2 {
		return 0
	}
	mean, std := meanStd(returns)
	if std == 0 {
		return 0
	}
	periodsPerYear := float64(periodsPerYear(granularity))
	return (mean / std) * math.Sqrt(periodsPerYear)
}

// calcSortino computes the annualized Sortino ratio.
func calcSortino(returns []float64, granularity string) float64 {
	if len(returns) < 2 {
		return 0
	}
	mean, _ := meanStd(returns)
	var downVariance float64
	count := 0
	for _, r := range returns {
		if r < 0 {
			downVariance += r * r
			count++
		}
	}
	if count == 0 {
		return 0
	}
	downStd := math.Sqrt(downVariance / float64(count))
	if downStd == 0 {
		return 0
	}
	periodsPerYear := float64(periodsPerYear(granularity))
	return (mean / downStd) * math.Sqrt(periodsPerYear)
}

// calcMaxDrawdown returns the maximum peak-to-trough drawdown as a negative fraction.
func calcMaxDrawdown(portfolio []float64) float64 {
	if len(portfolio) == 0 {
		return 0
	}
	peak := portfolio[0]
	maxDD := 0.0
	for _, v := range portfolio {
		if v > peak {
			peak = v
		}
		dd := (v - peak) / peak
		if dd < maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// annualizedReturn converts a total return over N periods to an annualized rate.
func annualizedReturn(totalReturn float64, nPeriods int, granSec int64) float64 {
	if nPeriods <= 0 || granSec <= 0 {
		return 0
	}
	secondsPerYear := float64(365 * 24 * 3600)
	totalSeconds := float64(nPeriods) * float64(granSec)
	years := totalSeconds / secondsPerYear
	if years <= 0 {
		return 0
	}
	return math.Pow(1+totalReturn, 1/years) - 1
}

func periodsPerYear(granularity string) int {
	switch granularity {
	case "1m":
		return 365 * 24 * 60
	case "5m":
		return 365 * 24 * 12
	case "15m":
		return 365 * 24 * 4
	case "30m":
		return 365 * 24 * 2
	case "1h":
		return 365 * 24
	case "2h":
		return 365 * 12
	case "6h":
		return 365 * 4
	case "1d":
		return 365
	default:
		return 365 * 24
	}
}

func granularityToSeconds(g string) int64 {
	switch g {
	case "1m":
		return 60
	case "5m":
		return 5 * 60
	case "15m":
		return 15 * 60
	case "30m":
		return 30 * 60
	case "1h":
		return 3600
	case "2h":
		return 2 * 3600
	case "6h":
		return 6 * 3600
	case "1d":
		return 24 * 3600
	default:
		return 3600
	}
}

func meanStd(vals []float64) (float64, float64) {
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var variance float64
	for _, v := range vals {
		diff := v - mean
		variance += diff * diff
	}
	std := math.Sqrt(variance / float64(len(vals)))
	return mean, std
}

func availableSignalNames() string {
	names := make([]string, 0, len(AllSignals()))
	for k := range AllSignals() {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
