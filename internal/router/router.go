package router

import (
	"github.com/beliaevke/go-musthave-diploma/internal/db/postgres"
	"github.com/beliaevke/go-musthave-diploma/internal/handlers/balance"
	"github.com/beliaevke/go-musthave-diploma/internal/handlers/orders"
	"github.com/beliaevke/go-musthave-diploma/internal/handlers/users"
	"github.com/beliaevke/go-musthave-diploma/internal/logger"
	"github.com/beliaevke/go-musthave-diploma/internal/middleware/auth"

	"github.com/go-chi/chi"
)

type Router struct {
	R *chi.Mux
}

func NewRouter(db *postgres.DB) *Router {

	r := chi.NewRouter()

	usersrepo := users.NewRepo(db)
	ordersrepo := orders.NewRepo(db)
	balancerepo := balance.NewRepo(db)

	// Require Logging
	r.Use(logger.WithLogging)

	// User Routes
	r.Group(func(r chi.Router) {
		r.Post("/api/user/register", users.UserRegisterHandler(usersrepo))
		r.Post("/api/user/login", users.UserLoginHandler(usersrepo))
	})

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
