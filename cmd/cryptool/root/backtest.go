package root

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"cryptool/internal/backtest"
	"cryptool/internal/config"
)

// NewBacktestCmd builds the top-level `backtest` command with `run` and `combo` sub-commands.
func NewBacktestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backtest",
		Short: "Backtest trading strategies against historical candle data",
	}
	cmd.AddCommand(newBacktestRunCmd())
	cmd.AddCommand(newBacktestComboCmd())
	return cmd
}

// newBacktestRunCmd implements `cryptool backtest run`.
func newBacktestRunCmd() *cobra.Command {
	var (
		product     string
		granularity string
		startDate   string
		endDate     string
		signals     string
		combination string
		threshold   float64
		feeRate     float64
		slippage    float64
		taxRate     float64
		minEdge     float64
		rrMin       float64
		save        bool
		showTrades  bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a single backtest for a product/signal combination",
		Long: `Loads candle data from the local database and simulates trading using one or
more combined signals. Reports performance metrics including Sharpe, Sortino,
max drawdown, win rate, and total return after fees, slippage, and taxes.

Available signals: rsi, macd, bbands, ema_cross

Example:
  cryptool backtest run --product BTC-USD --granularity 1h \
    --start 2024-01-01 --end 2024-12-31 \
    --signals rsi,macd --combination voting --fee-rate 0.001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			if product == "" {
				return fmt.Errorf("--product is required")
			}

			start, end, err := parseDateRange(startDate, endDate)
			if err != nil {
				return err
			}

			candles, err := backtest.LoadCandles(cmd.Context(), cfg.Database.URL, "coinbase", product, start, end)
			if err != nil {
				return fmt.Errorf("load candles: %w", err)
			}
			if len(candles) == 0 {
				return fmt.Errorf("no candles found for %s in range [%s, %s); run 'data fetch' first", product, start.Format("2006-01-02"), end.Format("2006-01-02"))
			}
			fmt.Printf("Loaded %d candles for %s (%s to %s)\n", len(candles), product, candles[0].Time.Format("2006-01-02"), candles[len(candles)-1].Time.Format("2006-01-02"))

			sigList := parseSignalList(signals)

			params := backtest.Params{
				Exchange:    "coinbase",
				ProductID:   product,
				Granularity: granularity,
				Start:       start,
				End:         end,
				Signals:     sigList,
				Combination: backtest.CombinationMethod(combination),
				Threshold:   threshold,
				FeeRate:     feeRate,
				Slippage:    slippage,
				TaxRate:     taxRate,
				MinEdge:     minEdge,
				RRMin:       rrMin,
			}

			result, err := backtest.Run(candles, params)
			if err != nil {
				return fmt.Errorf("backtest failed: %w", err)
			}

			printResult(cmd, result, showTrades)

			if save {
				if err := backtest.SaveResult(cmd.Context(), cfg.Database.URL, result); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to save result: %v\n", err)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Result saved to database.")
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&product, "product", "", "product ID, e.g. BTC-USD (required)")
	cmd.Flags().StringVar(&granularity, "granularity", "1h", "candle granularity: 1m, 5m, 15m, 30m, 1h, 2h, 6h, 1d")
	cmd.Flags().StringVar(&startDate, "start", "", "start date (YYYY-MM-DD or RFC3339); defaults to earliest candle")
	cmd.Flags().StringVar(&endDate, "end", "", "end date (YYYY-MM-DD or RFC3339); defaults to latest candle")
	cmd.Flags().StringVar(&signals, "signals", "", "comma-separated signals: rsi,macd,bbands,ema_cross (default: all)")
	cmd.Flags().StringVar(&combination, "combination", "voting", "signal combination method: voting, consensus, weighted")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.5, "voting threshold [0,1]: fraction of signals that must agree")
	cmd.Flags().Float64Var(&feeRate, "fee-rate", 0.001, "per-leg fee rate, e.g. 0.001 = 0.1%")
	cmd.Flags().Float64Var(&slippage, "slippage", 0.001, "estimated slippage per trade")
	cmd.Flags().Float64Var(&taxRate, "tax-rate", 0.30, "tax rate on net profits (short-term capital gains)")
	cmd.Flags().Float64Var(&minEdge, "min-edge", 0.005, "minimum net gain required to enter a trade (after fees)")
	cmd.Flags().Float64Var(&rrMin, "rr-min", 1.5, "minimum risk/reward ratio required to enter a trade")
	cmd.Flags().BoolVar(&save, "save", false, "save result to backtest_results table")
	cmd.Flags().BoolVar(&showTrades, "trades", false, "print individual trade log")
	return cmd
}

// newBacktestComboCmd implements `cryptool backtest combo` — tests all signal combinations.
func newBacktestComboCmd() *cobra.Command {
	var (
		product     string
		granularity string
		startDate   string
		endDate     string
		combination string
		threshold   float64
		feeRate     float64
		slippage    float64
		taxRate     float64
		minEdge     float64
		rrMin       float64
		topN        int
		save        bool
	)

	cmd := &cobra.Command{
		Use:   "combo",
		Short: "Test all signal combinations and rank by Sharpe ratio",
		Long: `Loads candle data from the local database, then exhaustively tests every
non-empty subset of available signals (rsi, macd, bbands, ema_cross) and ranks
the results by annualized Sharpe ratio. Useful for finding the best combination
without overfitting—review out-of-sample metrics before trading live.

Example:
  cryptool backtest combo --product BTC-USD --granularity 1h \
    --start 2023-01-01 --end 2023-12-31 --top 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			if product == "" {
				return fmt.Errorf("--product is required")
			}

			start, end, err := parseDateRange(startDate, endDate)
			if err != nil {
				return err
			}

			candles, err := backtest.LoadCandles(cmd.Context(), cfg.Database.URL, "coinbase", product, start, end)
			if err != nil {
				return fmt.Errorf("load candles: %w", err)
			}
			if len(candles) == 0 {
				return fmt.Errorf("no candles found for %s in range [%s, %s); run 'data fetch' first", product, start.Format("2006-01-02"), end.Format("2006-01-02"))
			}
			fmt.Printf("Loaded %d candles for %s (%s to %s)\n\n", len(candles), product, candles[0].Time.Format("2006-01-02"), candles[len(candles)-1].Time.Format("2006-01-02"))

			allSignalNames := []string{"rsi", "macd", "bbands", "ema_cross"}
			combos := powerSet(allSignalNames)

			type ranked struct {
				result backtest.Result
				label  string
			}
			var results []ranked

			for _, combo := range combos {
				params := backtest.Params{
					Exchange:    "coinbase",
					ProductID:   product,
					Granularity: granularity,
					Start:       start,
					End:         end,
					Signals:     combo,
					Combination: backtest.CombinationMethod(combination),
					Threshold:   threshold,
					FeeRate:     feeRate,
					Slippage:    slippage,
					TaxRate:     taxRate,
					MinEdge:     minEdge,
					RRMin:       rrMin,
				}
				r, err := backtest.Run(candles, params)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: %v\n", strings.Join(combo, "+"), err)
					continue
				}
				results = append(results, ranked{result: r, label: strings.Join(combo, "+")})
			}

			// Sort by Sharpe descending
			sort.Slice(results, func(i, j int) bool {
				return results[i].result.Sharpe > results[j].result.Sharpe
			})

			limit := topN
			if limit <= 0 || limit > len(results) {
				limit = len(results)
			}

			fmt.Printf("%-30s %8s %8s %10s %10s %8s %8s\n",
				"Signals", "Sharpe", "Sortino", "MaxDD", "TotalRet", "WinRate", "Trades")
			fmt.Println(strings.Repeat("-", 90))
			for i := 0; i < limit; i++ {
				r := results[i].result
				fmt.Printf("%-30s %8.3f %8.3f %10.2f%% %9.2f%% %7.1f%% %7d\n",
					results[i].label,
					r.Sharpe,
					r.Sortino,
					r.MaxDrawdown*100,
					r.TotalReturn*100,
					r.WinRate*100,
					r.NumTrades,
				)
			}

			if save {
				saved := 0
				for i := 0; i < limit; i++ {
					if err := backtest.SaveResult(cmd.Context(), cfg.Database.URL, results[i].result); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: save %s: %v\n", results[i].label, err)
					} else {
						saved++
					}
				}
				fmt.Printf("\nSaved %d results to database.\n", saved)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&product, "product", "", "product ID, e.g. BTC-USD (required)")
	cmd.Flags().StringVar(&granularity, "granularity", "1h", "candle granularity: 1m, 5m, 15m, 30m, 1h, 2h, 6h, 1d")
	cmd.Flags().StringVar(&startDate, "start", "", "start date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&endDate, "end", "", "end date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&combination, "combination", "voting", "signal combination method: voting, consensus, weighted")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.5, "voting threshold [0,1]")
	cmd.Flags().Float64Var(&feeRate, "fee-rate", 0.001, "per-leg fee rate")
	cmd.Flags().Float64Var(&slippage, "slippage", 0.001, "estimated slippage per trade")
	cmd.Flags().Float64Var(&taxRate, "tax-rate", 0.30, "tax rate on net profits")
	cmd.Flags().Float64Var(&minEdge, "min-edge", 0.005, "minimum net edge to enter a trade")
	cmd.Flags().Float64Var(&rrMin, "rr-min", 1.5, "minimum risk/reward ratio")
	cmd.Flags().IntVar(&topN, "top", 10, "number of top results to display")
	cmd.Flags().BoolVar(&save, "save", false, "save top results to backtest_results table")
	return cmd
}

// printResult formats and prints a single backtest result.
func printResult(cmd *cobra.Command, r backtest.Result, showTrades bool) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintf(out, "=== Backtest Result: %s (%s) ===\n", r.ProductID, r.Granularity)
	fmt.Fprintf(out, "Period:       %s to %s\n", r.Start.Format("2006-01-02"), r.End.Format("2006-01-02"))
	fmt.Fprintf(out, "Signals:      %s\n", r.Signals)
	fmt.Fprintf(out, "Combination:  %s (threshold=%.2f)\n", r.Combination, r.Threshold)
	fmt.Fprintf(out, "Costs:        fee=%.3f%% slippage=%.3f%% tax=%.0f%%\n",
		r.FeeRate*100, r.Slippage*100, r.TaxRate*100)
	fmt.Fprintf(out, "Entry gate:   min_edge=%.3f%% rr_min=%.2f\n", r.MinEdge*100, r.RRMin)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Total return: %+.2f%%\n", r.TotalReturn*100)
	fmt.Fprintf(out, "Sharpe:       %.4f\n", r.Sharpe)
	fmt.Fprintf(out, "Sortino:      %.4f\n", r.Sortino)
	fmt.Fprintf(out, "Max drawdown: %.2f%%\n", r.MaxDrawdown*100)
	fmt.Fprintf(out, "Calmar:       %.4f\n", r.Calmar)
	fmt.Fprintf(out, "Win rate:     %.1f%%\n", r.WinRate*100)
	fmt.Fprintf(out, "# trades:     %d\n", r.NumTrades)

	if showTrades && len(r.Trades) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "%-20s %-20s %6s %12s %12s %8s\n",
			"Entry", "Exit", "Dir", "Entry$", "Exit$", "Net%")
		fmt.Fprintln(out, strings.Repeat("-", 82))
		for _, t := range r.Trades {
			dir := "LONG"
			if t.Direction == -1 {
				dir = "SHORT"
			}
			fmt.Fprintf(out, "%-20s %-20s %6s %12.4f %12.4f %+7.2f%%\n",
				t.EntryTime.Format("2006-01-02 15:04"),
				t.ExitTime.Format("2006-01-02 15:04"),
				dir,
				t.EntryPrice,
				t.ExitPrice,
				t.NetReturn*100,
			)
		}
	}
	fmt.Fprintln(out)
}

// parseDateRange parses optional start/end date strings; if empty, returns zero times
// (caller should resolve against candle data boundaries).
func parseDateRange(startStr, endStr string) (time.Time, time.Time, error) {
	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = parseDate(startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --start: %w", err)
		}
	} else {
		start = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	if endStr != "" {
		end, err = parseDate(endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --end: %w", err)
		}
	} else {
		end = time.Now().UTC()
	}

	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("--end must be after --start")
	}
	return start, end, nil
}

// parseSignalList splits a comma-separated signal string and trims whitespace.
func parseSignalList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// powerSet returns all non-empty subsets of the input slice.
func powerSet(items []string) [][]string {
	n := len(items)
	total := int(math.Pow(2, float64(n)))
	result := make([][]string, 0, total-1)
	for mask := 1; mask < total; mask++ {
		var subset []string
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				subset = append(subset, items[i])
			}
		}
		result = append(result, subset)
	}
	return result
}
