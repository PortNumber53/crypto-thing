package backtest

import "math"

// SignalValue represents a directional signal: +1 buy, -1 sell, 0 neutral.
type SignalValue int

const (
	Buy     SignalValue = 1
	Sell    SignalValue = -1
	Neutral SignalValue = 0
)

// SignalFunc is a function that produces a signal series from candles.
type SignalFunc func(candles []Candle) []SignalValue

// RSI computes the Relative Strength Index signal series.
// Buys when RSI < buyThresh (oversold), sells when RSI > sellThresh (overbought).
func RSI(period int, buyThresh, sellThresh float64) SignalFunc {
	return func(candles []Candle) []SignalValue {
		n := len(candles)
		out := make([]SignalValue, n)
		if n <= period {
			return out
		}

		gains := make([]float64, n)
		losses := make([]float64, n)
		for i := 1; i < n; i++ {
			delta := candles[i].Close - candles[i-1].Close
			if delta > 0 {
				gains[i] = delta
			} else {
				losses[i] = -delta
			}
		}

		// Initial average over first period
		var sumG, sumL float64
		for i := 1; i <= period; i++ {
			sumG += gains[i]
			sumL += losses[i]
		}
		avgG := sumG / float64(period)
		avgL := sumL / float64(period)

		rsiAt := func(ag, al float64) float64 {
			if al == 0 {
				return 100
			}
			rs := ag / al
			return 100 - (100 / (1 + rs))
		}

		for i := period; i < n; i++ {
			if i > period {
				avgG = (avgG*float64(period-1) + gains[i]) / float64(period)
				avgL = (avgL*float64(period-1) + losses[i]) / float64(period)
			}
			rsi := rsiAt(avgG, avgL)
			switch {
			case rsi < buyThresh:
				out[i] = Buy
			case rsi > sellThresh:
				out[i] = Sell
			default:
				out[i] = Neutral
			}
		}
		return out
	}
}

// MACD computes the MACD line vs signal line crossover signal.
// Buys when MACD crosses above signal line, sells when it crosses below.
func MACD(fast, slow, signal int) SignalFunc {
	return func(candles []Candle) []SignalValue {
		n := len(candles)
		out := make([]SignalValue, n)
		if n < slow+signal {
			return out
		}

		closes := make([]float64, n)
		for i, c := range candles {
			closes[i] = c.Close
		}

		emaFast := ema(closes, fast)
		emaSlow := ema(closes, slow)

		macdLine := make([]float64, n)
		for i := range macdLine {
			macdLine[i] = emaFast[i] - emaSlow[i]
		}

		sigLine := ema(macdLine, signal)

		for i := 1; i < n; i++ {
			prev := macdLine[i-1] - sigLine[i-1]
			curr := macdLine[i] - sigLine[i]
			switch {
			case prev <= 0 && curr > 0:
				out[i] = Buy
			case prev >= 0 && curr < 0:
				out[i] = Sell
			default:
				out[i] = Neutral
			}
		}
		return out
	}
}

// BollingerBands computes signals based on price touching the bands.
// Buys when price closes below lower band (oversold), sells when above upper band.
func BollingerBands(period int, numStdDev float64) SignalFunc {
	return func(candles []Candle) []SignalValue {
		n := len(candles)
		out := make([]SignalValue, n)
		if n < period {
			return out
		}

		for i := period - 1; i < n; i++ {
			var sum float64
			for j := i - period + 1; j <= i; j++ {
				sum += candles[j].Close
			}
			mean := sum / float64(period)

			var variance float64
			for j := i - period + 1; j <= i; j++ {
				diff := candles[j].Close - mean
				variance += diff * diff
			}
			stdDev := math.Sqrt(variance / float64(period))

			upper := mean + numStdDev*stdDev
			lower := mean - numStdDev*stdDev
			price := candles[i].Close

			switch {
			case price < lower:
				out[i] = Buy
			case price > upper:
				out[i] = Sell
			default:
				out[i] = Neutral
			}
		}
		return out
	}
}

// SMACross generates buy/sell signals when price crosses a simple moving average.
// Buys when price crosses above the SMA, sells when it crosses below.
func SMACross(period int) SignalFunc {
	return func(candles []Candle) []SignalValue {
		n := len(candles)
		out := make([]SignalValue, n)
		if n < period+1 {
			return out
		}

		// Compute running SMA
		sma := make([]float64, n)
		var sum float64
		for i := 0; i < period; i++ {
			sum += candles[i].Close
		}
		sma[period-1] = sum / float64(period)
		for i := period; i < n; i++ {
			sum += candles[i].Close - candles[i-period].Close
			sma[i] = sum / float64(period)
		}

		for i := period; i < n; i++ {
			prevAbove := candles[i-1].Close > sma[i-1]
			currAbove := candles[i].Close > sma[i]
			switch {
			case !prevAbove && currAbove:
				out[i] = Buy
			case prevAbove && !currAbove:
				out[i] = Sell
			default:
				out[i] = Neutral
			}
		}
		return out
	}
}

// EMACross generates buy/sell signals from a fast/slow EMA crossover.
func EMACross(fast, slow int) SignalFunc {
	return func(candles []Candle) []SignalValue {
		n := len(candles)
		out := make([]SignalValue, n)
		if n < slow+1 {
			return out
		}

		closes := make([]float64, n)
		for i, c := range candles {
			closes[i] = c.Close
		}

		emaFast := ema(closes, fast)
		emaSlow := ema(closes, slow)

		for i := 1; i < n; i++ {
			prevDiff := emaFast[i-1] - emaSlow[i-1]
			currDiff := emaFast[i] - emaSlow[i]
			switch {
			case prevDiff <= 0 && currDiff > 0:
				out[i] = Buy
			case prevDiff >= 0 && currDiff < 0:
				out[i] = Sell
			default:
				out[i] = Neutral
			}
		}
		return out
	}
}

// VolatilityFilter passes through the base signal only when recent rolling
// volatility (stddev of returns) exceeds minVolatility. Returns Neutral otherwise.
// It is typically used to filter out low-conviction entries during flat markets.
func VolatilityFilter(base SignalFunc, period int, minVolatility float64) SignalFunc {
	return func(candles []Candle) []SignalValue {
		n := len(candles)
		baseSignals := base(candles)
		out := make([]SignalValue, n)
		if n < period+1 {
			return out
		}

		returns := make([]float64, n)
		for i := 1; i < n; i++ {
			if candles[i-1].Close != 0 {
				returns[i] = (candles[i].Close - candles[i-1].Close) / candles[i-1].Close
			}
		}

		for i := period; i < n; i++ {
			var sum float64
			for j := i - period + 1; j <= i; j++ {
				sum += returns[j]
			}
			mean := sum / float64(period)
			var variance float64
			for j := i - period + 1; j <= i; j++ {
				diff := returns[j] - mean
				variance += diff * diff
			}
			vol := math.Sqrt(variance / float64(period))
			if vol >= minVolatility {
				out[i] = baseSignals[i]
			}
		}
		return out
	}
}

// Regime represents the detected market regime for a candle.
type Regime int

const (
	RegimeNeutral Regime = 0
	RegimeBull    Regime = 1
	RegimeBear    Regime = -1
)

// DetectRegime classifies each candle into a market regime.
//   - Bull: price above slowSMA AND fastSMA above slowSMA (golden cross)
//   - Bear: price below slowSMA AND fastSMA below slowSMA (death cross)
//   - Neutral: everything else (choppy/transitional)
func DetectRegime(candles []Candle, fastPeriod, slowPeriod int) []Regime {
	n := len(candles)
	out := make([]Regime, n)
	if n < slowPeriod+1 {
		return out
	}

	closes := make([]float64, n)
	for i, c := range candles {
		closes[i] = c.Close
	}

	fastSMA := sma(closes, fastPeriod)
	slowSMA := sma(closes, slowPeriod)

	for i := slowPeriod; i < n; i++ {
		price := closes[i]
		switch {
		case price > slowSMA[i] && fastSMA[i] > slowSMA[i]:
			out[i] = RegimeBull
		case price < slowSMA[i] && fastSMA[i] < slowSMA[i]:
			out[i] = RegimeBear
		default:
			out[i] = RegimeNeutral
		}
	}
	return out
}

// sma computes a simple moving average series.
func sma(values []float64, period int) []float64 {
	n := len(values)
	out := make([]float64, n)
	if period <= 0 || n < period {
		return out
	}
	var sum float64
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	out[period-1] = sum / float64(period)
	for i := period; i < n; i++ {
		sum += values[i] - values[i-period]
		out[i] = sum / float64(period)
	}
	return out
}

// AllSignals returns a map of all built-in signals with default parameters.
func AllSignals() map[string]SignalFunc {
	return map[string]SignalFunc{
		"rsi":       RSI(14, 30, 70),
		"macd":      MACD(12, 26, 9),
		"bbands":    BollingerBands(20, 2.0),
		"ema_cross": EMACross(9, 21),
		"sma":       SMACross(200),
	}
}

// ema computes an exponential moving average series.
func ema(values []float64, period int) []float64 {
	n := len(values)
	out := make([]float64, n)
	if period <= 0 || n < period {
		return out
	}
	k := 2.0 / float64(period+1)

	// Seed with SMA of first `period` values
	var seed float64
	for i := 0; i < period; i++ {
		seed += values[i]
	}
	out[period-1] = seed / float64(period)

	for i := period; i < n; i++ {
		out[i] = values[i]*k + out[i-1]*(1-k)
	}
	return out
}
