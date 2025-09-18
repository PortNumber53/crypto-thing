package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
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
