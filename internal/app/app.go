package app

import (
	"context"
	"net/http"

	"musthave-diploma/internal/config"
	"musthave-diploma/internal/db/postgres"
	"musthave-diploma/internal/handlers/orders"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/router"
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
