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

	r := chi.NewRouter()

	// Require Logging
	r.Use(logger.WithLogging)

	// User Routes
	r.Group(func(r chi.Router) {
		r.Post("/api/user/register", users.UserRegisterHandler(db))
		r.Post("/api/user/login", users.UserLoginHandler(db))
	})

	ordersrepo := orders.NewRepo(db)
	balancerepo := balance.NewRepo(db)

	// Orders & Balance Routes
	// Require Authentication
	r.Group(func(r chi.Router) {
		r.Use(auth.WithAuthentication)
		r.Get("/api/user/orders", orders.GetOrdersHandler(ordersrepo))
		r.Post("/api/user/orders", orders.PostOrdersHandler(ordersrepo))
		r.Get("/api/user/balance", balance.GetBalanceHandler(balancerepo))
		r.Post("/api/user/balance/withdraw", balance.PostBalanceWithdrawHandler(balancerepo))
		r.Get("/api/user/withdrawals", balance.GetWithdrawalsHandler(balancerepo))
	})

	return &Router{R: r}
}
