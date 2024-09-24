package app

import (
	"context"
	"log"
	"net/http"

	"musthave-diploma/internal/config"
	"musthave-diploma/internal/db/migrations"
	"musthave-diploma/internal/db/postgres"
	"musthave-diploma/internal/handlers/balance"
	"musthave-diploma/internal/handlers/orders"
	"musthave-diploma/internal/handlers/users"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/middleware/auth"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run() {

	cfg := config.ParseFlags()
	ctx := context.Background()

	if cfg.FlagDatabaseURI != "" {

		err := migrations.InitDB(ctx, cfg.FlagDatabaseURI)
		if err != nil {
			logger.Warnf("InitDB fail: " + err.Error())
			return
		}

		dbpool, err := postgres.SetDB(ctx, cfg.FlagDatabaseURI)
		if err != nil {
			logger.Warnf("SetDB fail: " + err.Error())
			return
		}

		if err := appstart(cfg, dbpool); err != nil {
			log.Fatal(err)
		}

	} else {
		logger.Warnf("Database URI is empty")
		return
	}
}

func appstart(cfg config.ServerFlags, dbpool *pgxpool.Pool) error {

	logger.ServerRunningInfo(cfg.FlagRunAddr)

	go orders.CheckOrders(cfg.FlagASAddr, dbpool)

	mux := chi.NewMux()
	mux.Use(logger.WithLogging)

	mux.Handle("/api/user/register", users.UserRegisterHandler(dbpool))
	mux.Handle("/api/user/login", users.UserLoginHandler(dbpool))
	mux.Handle("/api/user/orders", auth.WithAuthentication(orders.GetOrdersHandler(dbpool)))
	mux.Handle("/api/user/balance", auth.WithAuthentication(balance.GetBalanceHandler(dbpool)))
	mux.Handle("/api/user/balance/withdraw", auth.WithAuthentication(balance.PostBalanceWithdrawHandler(dbpool)))
	mux.Handle("/api/user/withdrawals", auth.WithAuthentication(balance.GetWithdrawalsHandler(dbpool)))

	return http.ListenAndServe(cfg.FlagRunAddr, mux)
}
