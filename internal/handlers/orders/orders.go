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

		db := initDB()
		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "text/plain")
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
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()

			orders, err := db.GetOrders(ctx, dbpool, userID)
			if err != nil {
				//http.Error(w, err.Error(), http.StatusInternalServerError)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNoContent)
				json.NewEncoder(w).Encode(orders)
				return
			}
			if len(orders) == 0 {
				http.Error(w, "orders not found", http.StatusNoContent) // w.WriteHeader(http.StatusNoContent)
				return
			}
			fmt.Println("=========================Get orders 1")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(orders)
			fmt.Println(r.Body)
			fmt.Println("=========================Get orders end ")
		}
	}
	return http.HandlerFunc(fn)
}

func SendOrdersHandler(ctx context.Context, dbpool *pgxpool.Pool, FlagASAddr string, o repository.Order) error {

	client := &http.Client{}

	url := fmt.Sprintf("%s/api/orders/%v", FlagASAddr, o.OrderNumber)
	fmt.Println("!!=======================SendOrders " + url)
	var body []byte
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err = io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	fmt.Println("!!=======================SendOrders OrderNumber " + o.OrderNumber)
	orderUID, err := initDB().GetOrder(ctx, dbpool, o.OrderNumber)
	if err != nil {
		return err
	}
	fmt.Println("!!=======================SendOrders start")
	if response.StatusCode == http.StatusNoContent {
		o.OrderStatus = "INVALID"
		repository.UpdateOrder(ctx, dbpool, orderUID, o)
		return nil
	} else if response.StatusCode == http.StatusTooManyRequests {
		logger.Warnf("number of requests to the service has been exceeded")
		return err
	} else if response.StatusCode != http.StatusOK {
		logger.Warnf("send order for calculation error")
		return err
	}
	var respBody struct {
		Order   string  `json:"order"`
		Status  string  `json:"status"`
		Accrual float32 `json:"accrual"`
	}
	fmt.Println("!!=======================SendOrders respBody")
	err = json.Unmarshal(body, &respBody)
	if err != nil {
		logger.Warnf("unmarshal response body error")
		return err
	}
	if respBody.Status == "PROCESSED" {
		o.OrderStatus = respBody.Status
		o.Accrual = respBody.Accrual
	} else if respBody.Status == "INVALID" {
		o.OrderStatus = respBody.Status
	} else {
		o.OrderStatus = "PROCESSING"
	}
	fmt.Println("!!=======================SendOrders UpdateOrder")
	repository.UpdateOrder(ctx, dbpool, orderUID, o)
	return nil
}
