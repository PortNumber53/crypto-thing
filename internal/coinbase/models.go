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

// Product represents a single trading product from the Coinbase API.
type Product struct {
	ProductID                   string                      `json:"product_id"`
	Price                       string                      `json:"price"`
	PricePercentageChange24h    string                      `json:"price_percentage_change_24h"`
	Volume24h                   string                      `json:"volume_24h"`
	VolumePercentageChange24h   string                      `json:"volume_percentage_change_24h"`
	BaseIncrement               string                      `json:"base_increment"`
	QuoteIncrement              string                      `json:"quote_increment"`
	QuoteMinSize                string                      `json:"quote_min_size"`
	QuoteMaxSize                string                      `json:"quote_max_size"`
	BaseMinSize                 string                      `json:"base_min_size"`
	BaseMaxSize                 string                      `json:"base_max_size"`
	BaseName                    string                      `json:"base_name"`
	QuoteName                   string                      `json:"quote_name"`
	Watched                     bool                        `json:"watched"`
	IsDisabled                  bool                        `json:"is_disabled"`
	New                         bool                        `json:"new"`
	Status                      string                      `json:"status"`
	CancelOnly                  bool                        `json:"cancel_only"`
	LimitOnly                   bool                        `json:"limit_only"`
	PostOnly                    bool                        `json:"post_only"`
	TradingDisabled             bool                        `json:"trading_disabled"`
	AuctionMode                 bool                        `json:"auction_mode"`
	ProductType                 string                      `json:"product_type"`
	QuoteCurrencyID             string                      `json:"quote_currency_id"`
	BaseCurrencyID              string                      `json:"base_currency_id"`
	FcmTradingSessionDetails    *FcmTradingSessionDetails   `json:"fcm_trading_session_details,omitempty"`
	MidMarketPrice              string                      `json:"mid_market_price"`
	Alias                       string                      `json:"alias"`
	AliasTo                     []string                    `json:"alias_to"`
	BaseDisplaySymbol           string                      `json:"base_display_symbol"`
	QuoteDisplaySymbol          string                      `json:"quote_display_symbol"`
	ViewOnly                    bool                        `json:"view_only"`
	PriceIncrement              string                      `json:"price_increment"`
	DisplayName                 string                      `json:"display_name"`
	ProductVenue                string                      `json:"product_venue"`
	ApproximateQuote24hVolume   string                      `json:"approximate_quote_24h_volume"`
	NewAt                       time.Time                   `json:"new_at"`
	FutureProductDetails        *FutureProductDetails       `json:"future_product_details,omitempty"`
}

type FcmTradingSessionDetails struct {
	IsSessionOpen                 bool         `json:"is_session_open"`
	OpenTime                      string       `json:"open_time"`
	CloseTime                     string       `json:"close_time"`
	SessionState                  string       `json:"session_state"`
	AfterHoursOrderEntryDisabled  bool         `json:"after_hours_order_entry_disabled"`
	ClosedReason                  string       `json:"closed_reason"`
	Maintenance                   *Maintenance `json:"maintenance,omitempty"`
}

type Maintenance struct {
	StartTme string `json:"start_time"`
	EndTime  string `json:"end_time"`
}

type FutureProductDetails struct {
	Venue                   string             `json:"venue"`
	ContractCode            string             `json:"contract_code"`
	ContractExpiry          string             `json:"contract_expiry"`
	ContractSize            string             `json:"contract_size"`
	ContractRootUnit        string             `json:"contract_root_unit"`
	GroupDescription        string             `json:"group_description"`
	ContractExpiryTimezone  string             `json:"contract_expiry_timezone"`
	GroupShortDescription   string             `json:"group_short_description"`
	RiskManagedBy           string             `json:"risk_managed_by"`
	ContractExpiryType      string             `json:"contract_expiry_type"`
	PerpetualDetails        *PerpetualDetails  `json:"perpetual_details,omitempty"`
	ContractDisplayName     string             `json:"contract_display_name"`
	TimeToExpiryMs          string             `json:"time_to_expiry_ms"`
	NonCrypto               bool               `json:"non_crypto"`
	ContractExpiryName      string             `json:"contract_expiry_name"`
	TwentyFourBySeven       bool               `json:"twenty_four_by_seven"`
	FundingInterval         string             `json:"funding_interval"`
	OpenInterest            string             `json:"open_interest"`
	FundingRate             string             `json:"funding_rate"`
	FundingTime             string             `json:"funding_time"`
}

type PerpetualDetails struct {
	OpenInterest    string `json:"open_interest"`
	FundingRate     string `json:"funding_rate"`
	FundingTime     string `json:"funding_time"`
	MaxLeverage     string `json:"max_leverage"`
	BaseAssetUUID   string `json:"base_asset_uuid"`
	UnderlyingType  string `json:"underlying_type"`
}

// ListProductsResponse is the response from the /market/products endpoint.
type ListProductsResponse struct {
	Products    []Product `json:"products"`
	NumProducts int       `json:"num_products"`
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
