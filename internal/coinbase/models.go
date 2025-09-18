package coinbase

import "time"

type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type ListAccountsResponse struct {
	Accounts []Account `json:"accounts"`
	HasNext  bool      `json:"has_next"`
	Cursor   string    `json:"cursor"`
	Size     int       `json:"size"`
}

type Account struct {
	UUID             string  `json:"uuid"`
	Name             string  `json:"name"`
	Currency         string  `json:"currency"`
	AvailableBalance Balance `json:"available_balance"`
	Default          bool    `json:"default"`
	Active           bool    `json:"active"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	DeletedAt        string  `json:"deleted_at,omitempty"`
	Type             string  `json:"type"`
	Ready            bool    `json:"ready"`
	Hold             Balance `json:"hold"`
}

type Balance struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}
