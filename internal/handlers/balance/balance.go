package balance

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/repository/balancerepo"

	"github.com/ShiraazMoollatjie/goluhn"
	"github.com/avast/retry-go/v4"
	"github.com/jackc/pgx/v5/pgxpool"
)

type database interface {
	GetBalance(ctx context.Context, dbpool *pgxpool.Pool, userID int) (balancerepo.Balance, error)
	BalanceWithdraw(ctx context.Context, dbpool *pgxpool.Pool, userID int, userBalance balancerepo.Balance, withdraw balancerepo.Withdraw) error
	GetWithdrawals(ctx context.Context, dbpool *pgxpool.Pool, userID int) ([]balancerepo.Withdrawals, error)
}

func initDB() database {
	return balancerepo.NewBalance()
}

func GetBalanceHandler(dbpool *pgxpool.Pool) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		db := initDB()
		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodGet {

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			balance, err := db.GetBalance(ctx, dbpool, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			encoder := json.NewEncoder(w)
			err = encoder.Encode(&balance)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}
	return http.HandlerFunc(fn)
}

func PostBalanceWithdrawHandler(dbpool *pgxpool.Pool) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		db := initDB()
		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodPost {

			var buf bytes.Buffer
			// читаем тело запроса
			n, err := buf.ReadFrom(r.Body)
			if err != nil || n == 0 {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			var withdraw balancerepo.Withdraw
			err = retry.Do(func() error {
				// десериализуем JSON в Visitor
				if err = json.Unmarshal(buf.Bytes(), &withdraw); err != nil {
					return err
				}
				return nil
			},
				retry.Attempts(3),
				retry.Delay(1000*time.Millisecond),
			)
			if err != nil {
				logger.Warnf("JSON error: " + err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			err = goluhn.Validate(withdraw.OrderNumber)
			if err != nil {
				logger.Warnf("goluhn validate error: " + err.Error())
				http.Error(w, "incorrect order number", http.StatusUnprocessableEntity)
				return
			}

			userBalance, err := db.GetBalance(ctx, dbpool, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if userBalance.PointsSum < withdraw.Sum {
				http.Error(w, "there are insufficient funds in the account", http.StatusPaymentRequired)
				return
			}

			err = db.BalanceWithdraw(ctx, dbpool, userID, userBalance, withdraw)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		}
	}
	return http.HandlerFunc(fn)
}

func GetWithdrawalsHandler(dbpool *pgxpool.Pool) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		db := initDB()
		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodGet {

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			withdrawals, err := db.GetWithdrawals(ctx, dbpool, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(withdrawals) == 0 {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			encoder := json.NewEncoder(w)
			err = encoder.Encode(&withdrawals)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}
	return http.HandlerFunc(fn)
}
