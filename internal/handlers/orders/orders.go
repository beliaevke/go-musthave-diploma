package orders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/repository"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ShiraazMoollatjie/goluhn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type database interface {
	AddOrder(ctx context.Context, dbpool *pgxpool.Pool, userID int, orderNumber string) error
	GetOrder(ctx context.Context, dbpool *pgxpool.Pool, orderNumber string) (int, error)
	GetOrders(ctx context.Context, dbpool *pgxpool.Pool, userID int) ([]repository.Order, error)
}

func initDB() database {
	return repository.NewOrder()
}

func GetOrdersHandler(dbpool *pgxpool.Pool) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
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
			responseString := buf.String()
			err = goluhn.Validate(responseString)
			if err != nil {
				logger.Warnf("goluhn validate error: " + err.Error())
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			orderUID, err := db.GetOrder(ctx, dbpool, responseString)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else if userID == orderUID && orderUID != -1 {
				w.WriteHeader(http.StatusOK)
				return
			} else if userID != orderUID && orderUID != -1 {
				http.Error(w, "order already exists with another user", http.StatusConflict)
				return
			}

			err = db.AddOrder(ctx, dbpool, userID, responseString)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusAccepted)
		} else if r.Method == http.MethodGet {

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			orders, err := db.GetOrders(ctx, dbpool, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(orders) == 0 {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			encoder := json.NewEncoder(w)
			err = encoder.Encode(&orders)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}
	return http.HandlerFunc(fn)
}

func SendOrdersHandler(dbpool *pgxpool.Pool, FlagASAddr string) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		orders, err := repository.GetAwaitOrders(ctx, dbpool)
		if err != nil {
			logger.Warnf(err.Error())
			return
		}

		client := &http.Client{}
		for _, o := range orders {
			url := fmt.Sprintf("%s/api/orders/%v", FlagASAddr, o.OrderNumber)
			var body []byte
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewBuffer(body))
			if err != nil {
				return
			}
			response, err := client.Do(request)
			if err != nil {
				return
			}
			response.Body.Close()
			_, err = io.Copy(os.Stdout, response.Body)
			if err != nil {
				return
			}
			orderUID, err := initDB().GetOrder(ctx, dbpool, o.OrderNumber)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if response.StatusCode == http.StatusNoContent {
				o.OrderStatus = "INVALID"
				repository.UpdateOrder(ctx, dbpool, orderUID, o)
				return
			} else if response.StatusCode == http.StatusTooManyRequests {
				logger.Warnf("number of requests to the service has been exceeded")
				return
			} else if response.StatusCode != http.StatusOK {
				logger.Warnf("send order for calculation error")
				return
			}
			var respBody struct {
				Order   string  `json:"order"`
				Status  string  `json:"status"`
				Accrual float32 `json:"accrual"`
			}
			err = json.Unmarshal(body, &respBody)
			if err != nil {
				logger.Warnf("unmarshal response body error")
				return
			}
			if respBody.Status == "PROCESSED" {
				o.OrderStatus = respBody.Status
				o.Accrual = respBody.Accrual
			} else if respBody.Status == "INVALID" {
				o.OrderStatus = respBody.Status
			} else {
				o.OrderStatus = "PROCESSING"
			}
			repository.UpdateOrder(ctx, dbpool, orderUID, o)
		}

	}
	return http.HandlerFunc(fn)
}
