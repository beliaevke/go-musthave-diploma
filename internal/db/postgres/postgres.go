package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func SetDB(ctx context.Context, databaseURI string) (*pgxpool.Pool, error) {
	dbpool, err := pgxpool.New(ctx, databaseURI)
	if err != nil {
		return dbpool, err
	}
	return dbpool, nil
}
