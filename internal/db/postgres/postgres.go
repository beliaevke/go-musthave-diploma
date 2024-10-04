package postgres

import (
	"context"
	"time"

	"github.com/beliaevke/go-musthave-diploma/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	Pool           *pgxpool.Pool
	DefaultTimeout time.Duration
}

func NewDB(ctx context.Context, cfg config.ServerFlags) (*DB, error) {
	dbpool, err := pgxpool.New(ctx, cfg.FlagDatabaseURI)
	if err != nil {
		return &DB{}, err
	}
	return &DB{Pool: dbpool, DefaultTimeout: cfg.DefaultTimeout}, nil
}
