package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"musthave-diploma/internal/config"
	"musthave-diploma/internal/db/postgres"
	"musthave-diploma/internal/handlers/balance"
	"musthave-diploma/internal/handlers/orders"
	"musthave-diploma/internal/handlers/users"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/middleware/authentication"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	fmt.Fprintln(os.Stdout, "start  main")
	fmt.Fprintln(os.Stdout, "start  flags")
	cfg := config.ParseFlags()
	fmt.Fprintln(os.Stdout, "start  ctx")
	ctx := context.Background()
	fmt.Fprintln(os.Stdout, "start  FlagDatabaseURI")
	if cfg.FlagDatabaseURI != "" {
		/*err := migrations.InitDB(ctx, cfg.FlagDatabaseURI)
		if err != nil {
			logger.Warnf("InitDB fail: " + err.Error())
		}*/
		dbpool, err := postgres.SetDB(ctx, cfg.FlagDatabaseURI)
		if err != nil {
			logger.Warnf("SetDB fail: " + err.Error())
		}
		if err := run(cfg, dbpool); err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Fprintln(os.Stdout, "Database URI is empty")
		logger.Warnf("Database URI is empty")
	}
}

func run(cfg config.ServerFlags, dbpool *pgxpool.Pool) error {
	logger.ServerRunningInfo(cfg.FlagRunAddr)

	mux := chi.NewMux()
	mux.Use(logger.WithLogging)

	mux.Handle("/api/user/register", users.UserRegisterHandler(dbpool))
	mux.Handle("/api/user/login", users.UserLoginHandler(dbpool))
	mux.Handle("/api/user/orders", authentication.WithAuthentication(orders.GetOrdersHandler(dbpool)))
	mux.Handle("/api/user/balance", authentication.WithAuthentication(balance.GetBalanceHandler(dbpool)))
	mux.Handle("/api/user/balance/withdraw", authentication.WithAuthentication(balance.PostBalanceWithdrawHandler(dbpool)))
	mux.Handle("/api/user/withdrawals", authentication.WithAuthentication(balance.GetWithdrawalsHandler(dbpool)))

	return http.ListenAndServe(cfg.FlagRunAddr, mux)
}
