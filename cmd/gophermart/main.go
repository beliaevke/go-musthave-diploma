package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"musthave-diploma/internal/config"
	"musthave-diploma/internal/db/migrations"
	"musthave-diploma/internal/db/postgres"
	"musthave-diploma/internal/handlers/balance"
	"musthave-diploma/internal/handlers/orders"
	"musthave-diploma/internal/handlers/users"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/middleware/authentication"
	"musthave-diploma/internal/repository"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.ParseFlags()
	ctx := context.Background()
	if cfg.FlagDatabaseURI != "" {
		err := migrations.InitDB(ctx, cfg.FlagDatabaseURI)
		if err != nil {
			logger.Warnf("InitDB fail: " + err.Error())
		}
		dbpool, err := postgres.SetDB(ctx, cfg.FlagDatabaseURI)
		if err != nil {
			logger.Warnf("SetDB fail: " + err.Error())
		}
		if err := run(cfg, dbpool); err != nil {
			log.Fatal(err)
		}
	} else {
		logger.Warnf("Database URI is empty")
	}
}

func run(cfg config.ServerFlags, dbpool *pgxpool.Pool) error {

	logger.ServerRunningInfo(cfg.FlagRunAddr)

	go checkOrders(cfg, dbpool)

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

func checkOrders(cfg config.ServerFlags, dbpool *pgxpool.Pool) {

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fmt.Println("=======!!!=============== checkOrders start")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("=======!!!=============== checkOrders Done")
			return
		case <-ticker.C:
			fmt.Println("=======!!!=============== checkOrders C")
			AwaitOrders, err := repository.GetAwaitOrders(ctx, dbpool)
			if err != nil {
				logger.Warnf(err.Error())
				fmt.Println("=======!!!=============== checkOrders continue 1")
				continue
			}
			if len(AwaitOrders) == 0 {
				fmt.Println("=======!!!=============== checkOrders continue 1")
				continue
			}
			for _, order := range AwaitOrders {
				err = orders.SendOrdersHandler(ctx, dbpool, cfg.FlagASAddr, order)
				if err != nil {
					logger.Warnf(err.Error())
				}
			}
			fmt.Println("=======!!!=============== checkOrders continue 2")
			continue
		}
	}
}
