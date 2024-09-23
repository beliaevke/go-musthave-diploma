package migrations

import (
	"context"
	"database/sql"
	"embed"
	"musthave-diploma/internal/logger"
	"time"

	"github.com/pressly/goose/v3"
)

//go:embed sql/*.sql
var embedMigrations embed.FS

func InitDB(ctx context.Context, databaseURI string) error {
	db, err := sql.Open("pgx", databaseURI)
	if err != nil {
		logger.Warnf("sql.Open(): " + err.Error())
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warnf("goose: failed to close DB: " + err.Error())
		}
	}()
	goose.SetBaseFS(embedMigrations)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := goose.UpContext(ctx, db, "sql"); err != nil {
		logger.Warnf("goose up: run failed  " + err.Error())
	}
	return err
}
