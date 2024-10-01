package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	Pool *pgxpool.Pool
}

func NewDB(ctx context.Context, databaseURI string) (*DB, error) {
	dbpool, err := pgxpool.New(ctx, databaseURI)
	if err != nil {
		return &DB{}, err
	}
	return &DB{Pool: dbpool}, nil
}
