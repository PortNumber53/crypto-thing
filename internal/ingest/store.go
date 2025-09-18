package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/lib/pq"
	"cryptool/internal/coinbase"
)

type Store struct {
	url string
}

// CountCandlesInRange returns how many candles exist for an exchange/product in [start, end).
func (s *Store) CountCandlesInRange(ctx context.Context, exchange, product string, start, end time.Time) (int, error) {
    db, err := sql.Open("postgres", s.url)
    if err != nil {
        return 0, err
    }
    defer db.Close()

    var cnt int
    err = db.QueryRowContext(ctx, `
        SELECT COUNT(*)
        FROM candles
        WHERE exchange = $1 AND product_id = $2 AND time >= $3 AND time < $4
    `, exchange, product, start, end).Scan(&cnt)
    if err != nil {
        return 0, err
    }
    return cnt, nil
}

func NewStore(url string) *Store {
	return &Store{url: url}
}

func (s *Store) UpsertProducts(ctx context.Context, exchange string, products []coinbase.Product) (int, error) {
	db, err := sql.Open("postgres", s.url)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO products(
			exchange, product_id, base_name, quote_name, is_disabled, price, 
			price_percentage_change_24h, volume_24h, volume_percentage_change_24h, 
			base_increment, quote_increment, quote_min_size, quote_max_size, 
			base_min_size, base_max_size, watched, is_new, status, cancel_only, 
			limit_only, post_only, trading_disabled, auction_mode, product_type, 
			quote_currency_id, base_currency_id, fcm_trading_session_details, 
			mid_market_price, alias, alias_to, base_display_symbol, 
			quote_display_symbol, view_only, price_increment, display_name, 
			product_venue, approximate_quote_24h_volume, new_at, future_product_details
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39)
		ON CONFLICT (exchange, product_id) DO UPDATE SET
			base_name = EXCLUDED.base_name,
			quote_name = EXCLUDED.quote_name,
			is_disabled = EXCLUDED.is_disabled,
			price = EXCLUDED.price,
			price_percentage_change_24h = EXCLUDED.price_percentage_change_24h,
			volume_24h = EXCLUDED.volume_24h,
			volume_percentage_change_24h = EXCLUDED.volume_percentage_change_24h,
			base_increment = EXCLUDED.base_increment,
			quote_increment = EXCLUDED.quote_increment,
			quote_min_size = EXCLUDED.quote_min_size,
			quote_max_size = EXCLUDED.quote_max_size,
			base_min_size = EXCLUDED.base_min_size,
			base_max_size = EXCLUDED.base_max_size,
			watched = EXCLUDED.watched,
			is_new = EXCLUDED.is_new,
			status = EXCLUDED.status,
			cancel_only = EXCLUDED.cancel_only,
			limit_only = EXCLUDED.limit_only,
			post_only = EXCLUDED.post_only,
			trading_disabled = EXCLUDED.trading_disabled,
			auction_mode = EXCLUDED.auction_mode,
			product_type = EXCLUDED.product_type,
			quote_currency_id = EXCLUDED.quote_currency_id,
			base_currency_id = EXCLUDED.base_currency_id,
			fcm_trading_session_details = EXCLUDED.fcm_trading_session_details,
			mid_market_price = EXCLUDED.mid_market_price,
			alias = EXCLUDED.alias,
			alias_to = EXCLUDED.alias_to,
			base_display_symbol = EXCLUDED.base_display_symbol,
			quote_display_symbol = EXCLUDED.quote_display_symbol,
			view_only = EXCLUDED.view_only,
			price_increment = EXCLUDED.price_increment,
			display_name = EXCLUDED.display_name,
			product_venue = EXCLUDED.product_venue,
			approximate_quote_24h_volume = EXCLUDED.approximate_quote_24h_volume,
			new_at = EXCLUDED.new_at,
			future_product_details = EXCLUDED.future_product_details
	`)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	var rowsAffectedCount int64
	for _, p := range products {
		fcmDetails, err := json.Marshal(p.FcmTradingSessionDetails)
		if err != nil {
			return 0, fmt.Errorf("marshal fcm_trading_session_details for %s: %w", p.ProductID, err)
		}
		futureDetails, err := json.Marshal(p.FutureProductDetails)
		if err != nil {
			return 0, fmt.Errorf("marshal future_product_details for %s: %w", p.ProductID, err)
		}

		res, err := stmt.ExecContext(ctx, exchange, p.ProductID, p.BaseName, p.QuoteName, p.IsDisabled, 
			parseFloat(p.Price), parseFloat(p.PricePercentageChange24h), parseFloat(p.Volume24h), 
			parseFloat(p.VolumePercentageChange24h), parseFloat(p.BaseIncrement), parseFloat(p.QuoteIncrement), 
			parseFloat(p.QuoteMinSize), parseFloat(p.QuoteMaxSize), parseFloat(p.BaseMinSize), 
			parseFloat(p.BaseMaxSize), p.Watched, p.New, p.Status, p.CancelOnly, p.LimitOnly, p.PostOnly, 
			p.TradingDisabled, p.AuctionMode, p.ProductType, p.QuoteCurrencyID, p.BaseCurrencyID, 
			fcmDetails, parseFloat(p.MidMarketPrice), p.Alias, pq.Array(p.AliasTo), p.BaseDisplaySymbol, 
			p.QuoteDisplaySymbol, p.ViewOnly, parseFloat(p.PriceIncrement), p.DisplayName, p.ProductVenue, 
			parseFloat(p.ApproximateQuote24hVolume), p.NewAt, futureDetails)

		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("upsert product %s: %w", p.ProductID, err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("get rows affected for product %s: %w", p.ProductID, err)
		}
		rowsAffectedCount += rows
	}

	return int(rowsAffectedCount), tx.Commit()
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func (s *Store) InsertCandles(ctx context.Context, exchange, product string, candles []coinbase.Candle) (int, error) {
	db, err := sql.Open("postgres", s.url)
	if err != nil {
			return 0, err
		}
		defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO candles(exchange, product_id, time, open, high, low, close, volume)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (exchange, product_id, time) DO NOTHING`)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	var rowsAffectedCount int64
	for _, c := range candles {
		res, err := stmt.ExecContext(ctx, exchange, product, c.Time, c.Open, c.High, c.Low, c.Close, c.Volume)
		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("insert candle: %w", err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("get rows affected: %w", err)
		}
		rowsAffectedCount += rows
	}
	return int(rowsAffectedCount), tx.Commit()
}
