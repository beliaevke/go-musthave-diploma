package main

import (
	"context"
	"log"
	"net/http"

	"musthave-diploma/cmd/server/config"
	"musthave-diploma/handlers"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/postgres"

	"github.com/go-chi/chi"
)

func main() {
	cfg := config.ParseFlags()
	ctx := context.Background()
	if cfg.FlagDatabaseURI != "" {
		postgres.SetDB(ctx, cfg.FlagDatabaseURI)
	}
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg config.ServerFlags) error {
	logger.ServerRunningInfo(cfg.FlagRunAddr)

	mux := chi.NewMux()
	mux.Use(logger.WithLogging)

	mux.Handle("/api/user/register", handlers.UserRegisterHandler(cfg.FlagDatabaseURI))
	mux.Handle("/api/user/login", handlers.UserLoginHandler(cfg.FlagDatabaseURI))
	mux.Handle("/api/user/orders", handlers.WithAuthentication(handlers.GetOrdersHandler(cfg.FlagDatabaseURI)))
	mux.Handle("/api/user/balance", handlers.WithAuthentication(handlers.GetBalanceHandler(cfg.FlagDatabaseURI)))
	mux.Handle("/api/user/balance/withdraw", handlers.WithAuthentication(handlers.PostBalanceWithdrawHandler(cfg.FlagDatabaseURI)))
	mux.Handle("/api/user/withdrawals", handlers.WithAuthentication(handlers.GetWithdrawalsHandler(cfg.FlagDatabaseURI)))

	return http.ListenAndServe(cfg.FlagRunAddr, mux)
}
