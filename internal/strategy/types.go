// Package strategy provides definitions for trading strategies and signals.

package strategy

// Strategy defines a trading strategy with an ID and a description.
type Strategy struct {
    ID          string  `json:"id"`          // Unique identifier for the strategy
    Description string  `json:"description"` // Brief description of the strategy
}

// Signal defines a trading signal with a type and associated strategy ID.
type Signal struct {
    Type       string  `json:"type"`       // Type of signal (e.g., buy, sell)
    StrategyID string  `json:"strategy_id"` // ID of the strategy associated with this signal
}