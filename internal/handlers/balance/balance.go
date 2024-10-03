package balance

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"musthave-diploma/internal/db/postgres"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/repository/balancerepo"

	"github.com/ShiraazMoollatjie/goluhn"
	"github.com/avast/retry-go/v4"
)

type database interface {
	GetBalance(ctx context.Context, userID int) (balancerepo.Balance, error)
	BalanceWithdraw(ctx context.Context, userID int, userBalance balancerepo.Balance, withdraw balancerepo.Withdraw) error
	GetWithdrawals(ctx context.Context, userID int) ([]balancerepo.Withdrawals, error)
	Timeout() time.Duration
}

func NewRepo(db *postgres.DB) database {
	return balancerepo.NewBalance(db)
}

func GetBalanceHandler(repo database) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodGet {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), repo.Timeout())
		defer cancel()

		balance, err := repo.GetBalance(ctx, userID)
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
	return http.HandlerFunc(fn)
}

func PostBalanceWithdrawHandler(repo database) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodPost {
			return
		}

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

		ctx, cancel := context.WithTimeout(r.Context(), repo.Timeout())
		defer cancel()

		err = goluhn.Validate(withdraw.OrderNumber)
		if err != nil {
			logger.Infof("goluhn validate error: " + err.Error() + " - " + withdraw.OrderNumber)
			http.Error(w, "incorrect order number", http.StatusUnprocessableEntity)
			return
		}

		userBalance, err := repo.GetBalance(ctx, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if userBalance.PointsSum < withdraw.Sum {
			http.Error(w, "there are insufficient funds in the account", http.StatusPaymentRequired)
			return
		}

		err = repo.BalanceWithdraw(ctx, userID, userBalance, withdraw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
	return http.HandlerFunc(fn)
}

func GetWithdrawalsHandler(repo database) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), repo.Timeout())
		defer cancel()

		withdrawals, err := repo.GetWithdrawals(ctx, userID)
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
	return http.HandlerFunc(fn)
}
