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
	Adaptive  CombinationMethod = "adaptive"
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

	// Adaptive mode: separate signal sets per market regime.
	// When Combination is "adaptive", BullSignals run during bull regimes,
	// BearSignals during bear regimes, and Signals (or none) during neutral.
	BullSignals []string
	BearSignals []string
	RegimeFast  int // fast SMA period for regime detection (default 50)
	RegimeSlow  int // slow SMA period for regime detection (default 200)

	FeeRate      float64 // round-trip fee per leg, e.g. 0.001 = 0.1%
	Slippage     float64 // estimated slippage per trade, e.g. 0.001
	TaxRate      float64 // fraction of profits taxed, e.g. 0.30
	MinEdge      float64 // min expected net gain to enter a trade, e.g. 0.005
	RRMin        float64 // minimum risk/reward ratio, e.g. 1.5
	MaxLoss      float64 // max unrealized loss before force-closing, e.g. 0.05 = 5%; 0 = disabled
	ProfitGate   bool    // only exit positions when the trade is profitable
	PositionSize float64 // fixed dollar amount per trade, e.g. 1000; 0 = full-portfolio compounding
	Capital      float64 // total portfolio capital for risk metrics, e.g. 10000; 0 = use PositionSize or 1.0
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

// EquityPoint records mark-to-market portfolio equity at a candle boundary.
type EquityPoint struct {
	Time   time.Time
	Equity float64
	Price  float64
}

// Result holds the final performance metrics for a backtest run.
type Result struct {
	Exchange     string
	ProductID    string
	Granularity  string
	Start        time.Time
	End          time.Time
	Signals      string
	BullSignals  string
	BearSignals  string
	RegimeFast   int
	RegimeSlow   int
	Combination  string
	Threshold    float64
	FeeRate      float64
	Slippage     float64
	TaxRate      float64
	MinEdge      float64
	RRMin        float64
	MaxLoss      float64
	ProfitGate   bool
	PositionSize float64
	Capital      float64

	TotalReturn float64
	Sharpe      float64
	Sortino     float64
	MaxDrawdown float64
	Calmar      float64
	WinRate     float64
	NumTrades   int
	Trades      []Trade
	EquityCurve []EquityPoint
}

// position tracks an open trade.
type position struct {
	dir        int // +1 long, -1 short
	entryPrice float64
	entryTime  time.Time
}

// Run executes a backtest and returns the Result.
func Run(candles []Candle, p Params) (Result, error) {
	p = p.Normalized()
	if err := p.Validate(); err != nil {
		return Result{}, err
	}
	return runWithSignalSet(candles, p, AllSignals())
}

// runWithSignalSet is the deterministic engine seam used by unit tests.
func runWithSignalSet(candles []Candle, p Params, available map[string]SignalFunc) (Result, error) {
	if len(candles) < 30 {
		return Result{}, fmt.Errorf("need at least 30 candles, got %d", len(candles))
	}

	resolveFuncs := func(names []string) ([]SignalFunc, error) {
		var funcs []SignalFunc
		for _, name := range names {
			fn, ok := available[name]
			if !ok {
				return nil, fmt.Errorf("unknown signal %q; available: %s", name, availableSignalNames())
			}
			funcs = append(funcs, fn)
		}
		return funcs, nil
	}

	threshold := p.Threshold
	if threshold <= 0 {
		threshold = 0.5
	}

	var combined []SignalValue

	if p.Combination == Adaptive {
		// Adaptive mode: detect market regime and use different signals per regime.
		if len(p.BullSignals) == 0 || len(p.BearSignals) == 0 {
			return Result{}, fmt.Errorf("adaptive mode requires --bull-signals and --bear-signals")
		}

		regimeFast := p.RegimeFast
		if regimeFast <= 0 {
			regimeFast = 50
		}
		regimeSlow := p.RegimeSlow
		if regimeSlow <= 0 {
			regimeSlow = 200
		}
		regimes := DetectRegime(candles, regimeFast, regimeSlow)

		bullFuncs, err := resolveFuncs(p.BullSignals)
		if err != nil {
			return Result{}, err
		}
		bearFuncs, err := resolveFuncs(p.BearSignals)
		if err != nil {
			return Result{}, err
		}

		// Compute signal series for each regime set
		bullSigs := make([][]SignalValue, len(bullFuncs))
		for i, fn := range bullFuncs {
			bullSigs[i] = fn(candles)
		}
		bearSigs := make([][]SignalValue, len(bearFuncs))
		for i, fn := range bearFuncs {
			bearSigs[i] = fn(candles)
		}

		// Combine each set independently using voting
		bullCombined := combineSignals(bullSigs, Voting, threshold)
		bearCombined := combineSignals(bearSigs, Voting, threshold)

		// Also build neutral signals if Signals is set
		var neutralCombined []SignalValue
		if len(p.Signals) > 0 {
			neutralFuncs, err := resolveFuncs(p.Signals)
			if err != nil {
				return Result{}, err
			}
			neutralSigs := make([][]SignalValue, len(neutralFuncs))
			for i, fn := range neutralFuncs {
				neutralSigs[i] = fn(candles)
			}
			neutralCombined = combineSignals(neutralSigs, Voting, threshold)
		}

		// Select signal per candle based on regime
		combined = make([]SignalValue, len(candles))
		for i := range candles {
			switch regimes[i] {
			case RegimeBull:
				combined[i] = bullCombined[i]
			case RegimeBear:
				combined[i] = bearCombined[i]
			default:
				if neutralCombined != nil {
					combined[i] = neutralCombined[i]
				}
				// else Neutral → no signal
			}
		}
	} else {
		// Standard mode: single signal set
		var sigFuncs []SignalFunc
		if len(p.Signals) > 0 {
			var err error
			sigFuncs, err = resolveFuncs(p.Signals)
			if err != nil {
				return Result{}, err
			}
		}
		if len(sigFuncs) == 0 {
			for _, fn := range available {
				sigFuncs = append(sigFuncs, fn)
			}
		}

		allSigs := make([][]SignalValue, len(sigFuncs))
		for i, fn := range sigFuncs {
			allSigs[i] = fn(candles)
		}
		combined = combineSignals(allSigs, p.Combination, threshold)
	}

	// Determine granularity in seconds for expected-move window
	granSec := granularityToSeconds(p.Granularity)

	// Rolling volatility window for profitability gate (last 10 buckets)
	const volWindow = 10

	// Simulate trades
	fixedSize := p.PositionSize > 0
	initialCapital := 1.0
	if p.Capital > 0 {
		initialCapital = p.Capital
	} else if fixedSize {
		initialCapital = p.PositionSize
	}
	var (
		pos       *position
		portfolio = []float64{initialCapital}
		returns   []float64
		trades    []Trade
		equity    = []EquityPoint{{Time: candles[0].Time, Equity: initialCapital, Price: candles[0].Close}}
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

		// --- Stop-loss gate ---
		// Force-close if unrealized loss exceeds MaxLoss threshold.
		// updatePortfolio appends the new portfolio value after a trade.
		recordTrade := func(r float64) {
			if fixedSize {
				portfolio = append(portfolio, portfolio[len(portfolio)-1]+p.PositionSize*r)
			} else {
				portfolio = append(portfolio, portfolio[len(portfolio)-1]*(1+r))
			}
			returns = append(returns, r)
		}

		if pos != nil && p.MaxLoss > 0 {
			var unrealized float64
			if pos.dir == +1 {
				unrealized = (price - pos.entryPrice) / pos.entryPrice
			} else {
				unrealized = (pos.entryPrice - price) / pos.entryPrice
			}
			if unrealized <= -p.MaxLoss {
				t := closeTrade(pos, price, candles[i].Time, p.FeeRate, p.Slippage)
				trades = append(trades, t)
				recordTrade(t.NetReturn)
				pos = nil
				equity = append(equity, EquityPoint{Time: candles[i].Time, Equity: portfolio[len(portfolio)-1], Price: price})
				continue
			}
		}

		// --- Position management ---
		switch {
		case sig == Buy && pos == nil:
			// Enter long
			pos = &position{dir: +1, entryPrice: price, entryTime: candles[i].Time}

		case sig == Sell && pos == nil:
			// Enter short
			pos = &position{dir: -1, entryPrice: price, entryTime: candles[i].Time}

		case sig == Sell && pos != nil && pos.dir == +1:
			// Close long; if profit-gate is on, hold unless profitable
			t := closeTrade(pos, price, candles[i].Time, p.FeeRate, p.Slippage)
			if p.ProfitGate && t.NetReturn <= 0 {
				portfolio = append(portfolio, portfolio[len(portfolio)-1])
				break
			}
			trades = append(trades, t)
			recordTrade(t.NetReturn)
			pos = &position{dir: -1, entryPrice: price, entryTime: candles[i].Time}

		case sig == Buy && pos != nil && pos.dir == -1:
			// Close short; if profit-gate is on, hold unless profitable
			t := closeTrade(pos, price, candles[i].Time, p.FeeRate, p.Slippage)
			if p.ProfitGate && t.NetReturn <= 0 {
				portfolio = append(portfolio, portfolio[len(portfolio)-1])
				break
			}
			trades = append(trades, t)
			recordTrade(t.NetReturn)
			pos = &position{dir: +1, entryPrice: price, entryTime: candles[i].Time}

		case sig == Neutral && pos != nil:
			// Exit to flat; if profit-gate is on, hold unless profitable
			t := closeTrade(pos, price, candles[i].Time, p.FeeRate, p.Slippage)
			if p.ProfitGate && t.NetReturn <= 0 {
				portfolio = append(portfolio, portfolio[len(portfolio)-1])
				break
			}
			trades = append(trades, t)
			recordTrade(t.NetReturn)
			pos = nil

		default:
			portfolio = append(portfolio, portfolio[len(portfolio)-1])
		}
		equity = append(equity, EquityPoint{
			Time:   candles[i].Time,
			Equity: markToMarket(portfolio[len(portfolio)-1], pos, price, p, roundTripCost),
			Price:  price,
		})
	}

	// Close any open position at last price
	if pos != nil {
		last := candles[len(candles)-1]
		t := closeTrade(pos, last.Close, last.Time, p.FeeRate, p.Slippage)
		trades = append(trades, t)
		if fixedSize {
			portfolio = append(portfolio, portfolio[len(portfolio)-1]+p.PositionSize*t.NetReturn)
		} else {
			portfolio = append(portfolio, portfolio[len(portfolio)-1]*(1+t.NetReturn))
		}
		returns = append(returns, t.NetReturn)
		pos = nil
	}
	_ = pos

	// Apply tax on net gain
	finalValue := portfolio[len(portfolio)-1]
	totalReturn := (finalValue - initialCapital) / initialCapital
	if totalReturn > 0 && p.TaxRate > 0 {
		totalReturn = totalReturn * (1 - p.TaxRate)
		finalValue = initialCapital * (1 + totalReturn)
	}
	if len(equity) > 0 {
		equity[len(equity)-1].Equity = finalValue
	}

	// Metrics — annualize based on actual trades per year, not candle frequency.
	// When using fixed position sizing with a capital base, scale per-trade
	// returns to reflect the fraction of total capital at risk so that
	// Sharpe/Sortino measure portfolio-level risk, not per-position risk.
	durationYears := candles[len(candles)-1].Time.Sub(candles[0].Time).Hours() / (365.25 * 24)
	tradesPerYear := 0.0
	if durationYears > 0 && len(trades) > 0 {
		tradesPerYear = float64(len(trades)) / durationYears
	}
	riskReturns := returns
	if fixedSize && p.Capital > 0 {
		riskReturns = make([]float64, len(returns))
		scale := p.PositionSize / p.Capital
		for i, r := range returns {
			riskReturns[i] = r * scale
		}
	}
	sharpe := calcSharpe(riskReturns, tradesPerYear)
	sortino := calcSortino(riskReturns, tradesPerYear)
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
		Exchange:     p.Exchange,
		ProductID:    p.ProductID,
		Granularity:  p.Granularity,
		Start:        p.Start,
		End:          p.End,
		Signals:      strings.Join(p.Signals, ","),
		BullSignals:  strings.Join(p.BullSignals, ","),
		BearSignals:  strings.Join(p.BearSignals, ","),
		RegimeFast:   p.RegimeFast,
		RegimeSlow:   p.RegimeSlow,
		Combination:  string(p.Combination),
		Threshold:    p.Threshold,
		FeeRate:      p.FeeRate,
		Slippage:     p.Slippage,
		TaxRate:      p.TaxRate,
		MinEdge:      p.MinEdge,
		RRMin:        p.RRMin,
		MaxLoss:      p.MaxLoss,
		ProfitGate:   p.ProfitGate,
		PositionSize: p.PositionSize,
		Capital:      p.Capital,
		TotalReturn:  totalReturn,
		Sharpe:       sharpe,
		Sortino:      sortino,
		MaxDrawdown:  maxDD,
		Calmar:       calmar,
		WinRate:      winRate,
		NumTrades:    len(trades),
		Trades:       trades,
		EquityCurve:  equity,
	}, nil
}

func markToMarket(realized float64, pos *position, price float64, p Params, roundTripCost float64) float64 {
	if pos == nil || pos.entryPrice == 0 {
		return realized
	}
	raw := (price - pos.entryPrice) / pos.entryPrice
	if pos.dir == -1 {
		raw = -raw
	}
	net := raw - roundTripCost
	if p.PositionSize > 0 {
		return realized + p.PositionSize*net
	}
	return realized * (1 + net)
}

// SampleEquityCurve uniformly bounds a chart payload while preserving both
// endpoints. The input is copied even when no sampling is required.
func SampleEquityCurve(points []EquityPoint, limit int) []EquityPoint {
	if limit <= 0 || len(points) == 0 {
		return []EquityPoint{}
	}
	if len(points) <= limit {
		return append([]EquityPoint(nil), points...)
	}
	if limit == 1 {
		return []EquityPoint{points[len(points)-1]}
	}
	out := make([]EquityPoint, limit)
	for i := range out {
		index := i * (len(points) - 1) / (limit - 1)
		out[i] = points[index]
	}
	return out
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
// tradesPerYear is the actual trading frequency derived from the backtest period,
// not the candle granularity, so annualization is accurate regardless of timeframe.
func calcSharpe(returns []float64, tradesPerYear float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	mean, std := meanStd(returns)
	if std == 0 {
		return 0
	}
	return (mean / std) * math.Sqrt(tradesPerYear)
}

// calcSortino computes the annualized Sortino ratio.
// Downside deviation is computed over all returns (not just negative ones),
// treating non-negative returns as zero downside. This prevents the ratio
// from exploding when there are very few losing trades.
func calcSortino(returns []float64, tradesPerYear float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	mean, _ := meanStd(returns)
	var downVariance float64
	for _, r := range returns {
		if r < 0 {
			downVariance += r * r
		}
	}
	downStd := math.Sqrt(downVariance / float64(len(returns)))
	if downStd == 0 {
		return 0
	}
	return (mean / downStd) * math.Sqrt(tradesPerYear)
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
