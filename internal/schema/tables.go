package schema

import "strings"

// CandleTable returns the per-granularity candle table name, e.g. "candles_1h".
func CandleTable(granularity string) string {
	switch strings.ToLower(granularity) {
	case "1m":
		return "candles_1m"
	case "5m":
		return "candles_5m"
	case "15m":
		return "candles_15m"
	case "30m":
		return "candles_30m"
	case "1h":
		return "candles_1h"
	case "2h":
		return "candles_2h"
	case "6h":
		return "candles_6h"
	case "1d":
		return "candles_1d"
	default:
		return "candles_1h"
	}
}

// AllCandleTables returns the names of all per-granularity candle tables.
func AllCandleTables() []string {
	return []string{
		"candles_1m", "candles_5m", "candles_15m", "candles_30m",
		"candles_1h", "candles_2h", "candles_6h", "candles_1d",
	}
}

// GranularitySeconds maps a granularity string to the number of seconds per bucket.
func GranularitySeconds(granularity string) int64 {
	switch strings.ToLower(granularity) {
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
