package app

import (
	"context"
	"net/http"

	"github.com/beliaevke/go-musthave-diploma/internal/config"
	"github.com/beliaevke/go-musthave-diploma/internal/db/postgres"
	"github.com/beliaevke/go-musthave-diploma/internal/handlers/orders"
	"github.com/beliaevke/go-musthave-diploma/internal/logger"
	"github.com/beliaevke/go-musthave-diploma/internal/router"
)

func Run(cfg config.ServerFlags, ctx context.Context) error {

	db, err := postgres.NewDB(ctx, cfg)
	if err != nil {
		logger.Warnf("SetDB fail: " + err.Error())
		return err
	}

	logger.ServerRunningInfo(cfg.FlagRunAddr)

	go orders.CheckOrders(cfg.FlagASAddr, cfg.CheckOrdersTimeout, db)

	router := router.NewRouter(db)

	if err := http.ListenAndServe(cfg.FlagRunAddr, router.R); err != nil && err != http.ErrServerClosed {
		logger.Warnf("App start fail: " + err.Error())
		return err
	}

	return nil
}
