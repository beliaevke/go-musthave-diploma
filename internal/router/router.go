package router

import (
	"musthave-diploma/internal/db/postgres"
	"musthave-diploma/internal/handlers/balance"
	"musthave-diploma/internal/handlers/orders"
	"musthave-diploma/internal/handlers/users"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/middleware/auth"

	"github.com/go-chi/chi"
)

type Router struct {
	R *chi.Mux
}

func NewRouter(db *postgres.DB) *Router {

	mux := chi.NewMux()
	mux.Use(logger.WithLogging)

	mux.Handle("/api/user/register", users.UserRegisterHandler(db))
	mux.Handle("/api/user/login", users.UserLoginHandler(db))

	ordersrepo := orders.NewRepo()
	mux.Handle("/api/user/orders", auth.WithAuthentication(orders.GetOrdersHandler(db, ordersrepo)))

	balancerepo := balance.NewRepo()
	mux.Handle("/api/user/balance", auth.WithAuthentication(balance.GetBalanceHandler(db, balancerepo)))
	mux.Handle("/api/user/balance/withdraw", auth.WithAuthentication(balance.PostBalanceWithdrawHandler(db, balancerepo)))
	mux.Handle("GET /api/user/withdrawals", auth.WithAuthentication(balance.GetWithdrawalsHandler(db, balancerepo)))

	return &Router{R: mux}
}
