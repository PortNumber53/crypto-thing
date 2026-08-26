package backtest

import (
	"fmt"
	"sort"
)

// DefaultParams returns the canonical defaults shared by the CLI and REST API.
func DefaultParams() Params {
	return Params{
		Exchange:    "coinbase",
		Granularity: "1h",
		Combination: Voting,
		Threshold:   0.5,
		RegimeFast:  50,
		RegimeSlow:  200,
		FeeRate:     0.001,
		Slippage:    0.001,
		TaxRate:     0.30,
		MinEdge:     0.005,
		RRMin:       1.5,
	}
}

// Normalized fills structural defaults. Numeric trading values are intentionally
// left alone so callers can explicitly select zero fees, tax, or risk gates.
func (p Params) Normalized() Params {
	defaults := DefaultParams()
	if p.Exchange == "" {
		p.Exchange = defaults.Exchange
	}
	if p.Granularity == "" {
		p.Granularity = defaults.Granularity
	}
	if p.Combination == "" {
		p.Combination = defaults.Combination
	}
	if p.Threshold == 0 {
		p.Threshold = defaults.Threshold
	}
	if p.RegimeFast == 0 {
		p.RegimeFast = defaults.RegimeFast
	}
	if p.RegimeSlow == 0 {
		p.RegimeSlow = defaults.RegimeSlow
	}
	return p
}

// Validate rejects configurations that cannot be simulated consistently.
func (p Params) Validate() error {
	validGranularity := map[string]bool{
		"1m": true, "5m": true, "15m": true, "30m": true,
		"1h": true, "2h": true, "6h": true, "1d": true,
	}
	if !validGranularity[p.Granularity] {
		return fmt.Errorf("invalid granularity %q", p.Granularity)
	}
	switch p.Combination {
	case Voting, Consensus, Weighted, Adaptive:
	default:
		return fmt.Errorf("invalid combination %q", p.Combination)
	}
	if p.Threshold <= 0 || p.Threshold > 1 {
		return fmt.Errorf("threshold must be in (0,1]")
	}
	if p.FeeRate < 0 || p.FeeRate >= 1 {
		return fmt.Errorf("fee rate must be in [0,1)")
	}
	if p.Slippage < 0 || p.Slippage >= 1 {
		return fmt.Errorf("slippage must be in [0,1)")
	}
	if p.TaxRate < 0 || p.TaxRate > 1 {
		return fmt.Errorf("tax rate must be in [0,1]")
	}
	if p.MinEdge < 0 || p.MinEdge >= 1 {
		return fmt.Errorf("minimum edge must be in [0,1)")
	}
	if p.RRMin < 0 {
		return fmt.Errorf("minimum risk/reward must be non-negative")
	}
	if p.MaxLoss < 0 || p.MaxLoss >= 1 {
		return fmt.Errorf("maximum loss must be in [0,1)")
	}
	if p.PositionSize < 0 || p.Capital < 0 {
		return fmt.Errorf("position size and capital must be non-negative")
	}
	if p.PositionSize > 0 && p.Capital > 0 && p.PositionSize > p.Capital {
		return fmt.Errorf("position size cannot exceed capital")
	}
	if p.RegimeFast <= 0 || p.RegimeSlow <= 0 || p.RegimeFast >= p.RegimeSlow {
		return fmt.Errorf("regime periods must be positive with fast < slow")
	}
	if p.Combination == Adaptive && (len(p.BullSignals) == 0 || len(p.BearSignals) == 0) {
		return fmt.Errorf("adaptive mode requires bull and bear signals")
	}
	return nil
}

// SignalNames returns the built-in signal names in stable display order.
func SignalNames() []string {
	names := make([]string, 0, len(AllSignals()))
	for name := range AllSignals() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RunCombinations runs every non-empty built-in signal subset and returns the
// highest-Sharpe results. A non-positive top value uses the default of ten.
func RunCombinations(candles []Candle, base Params, top int) ([]Result, error) {
	names := SignalNames()
	results := make([]Result, 0, (1<<len(names))-1)
	for mask := 1; mask < 1<<len(names); mask++ {
		params := base
		params.Signals = make([]string, 0, len(names))
		for i, name := range names {
			if mask&(1<<i) != 0 {
				params.Signals = append(params.Signals, name)
			}
		}
		result, err := Run(candles, params)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Sharpe == results[j].Sharpe {
			return results[i].Signals < results[j].Signals
		}
		return results[i].Sharpe > results[j].Sharpe
	})
	if top <= 0 {
		top = 10
	}
	if top < len(results) {
		results = results[:top]
	}
	return results, nil
}
