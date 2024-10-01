package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"time"

	"musthave-diploma/internal/config"
	"musthave-diploma/internal/logger"

	"github.com/pressly/goose/v3"
)

//go:embed sql/*.sql
var embedMigrations embed.FS

func Run(cfg config.ServerFlags, ctx context.Context) error {

	if cfg.FlagDatabaseURI == "" {
		err := errors.New("Database URI is empty")
		logger.Warnf("InitDB fail: " + err.Error())
		return err
	}

	db, err := sql.Open("pgx", cfg.FlagDatabaseURI)
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
