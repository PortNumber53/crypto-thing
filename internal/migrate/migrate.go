package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func open(url string) (*sql.DB, error) {
	return sql.Open("postgres", url)
}

func Status(ctx context.Context, url string, migrationsFS embed.FS) error {
	db, err := open(url)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	fmt.Println("Migration status:")
	return goose.Status(db, "migrations")
}

func Up(ctx context.Context, url string, migrationsFS embed.FS) error {
	db, err := open(url)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}
	return nil
}

func Down(ctx context.Context, url string, steps int, migrationsFS embed.FS) error {
	db, err := open(url)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	if steps <= 0 {
		steps = 1
	}
	for i := 0; i < steps; i++ {
		if err := goose.Down(db, "migrations"); err != nil {
			return err
		}
	}
	return nil
}

func Reset(ctx context.Context, url string, migrationsFS embed.FS) error {
	db, err := open(url)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	if err := goose.Reset(db, "migrations"); err != nil {
		return err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}
	return nil
}
