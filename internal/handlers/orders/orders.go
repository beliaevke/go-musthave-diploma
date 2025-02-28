package orders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/beliaevke/go-musthave-diploma/internal/db/postgres"
	"github.com/beliaevke/go-musthave-diploma/internal/logger"
	"github.com/beliaevke/go-musthave-diploma/internal/repository/ordersrepo"

	"github.com/ShiraazMoollatjie/goluhn"
)

type database interface {
	AddOrder(ctx context.Context, userID int, orderNumber string) error
	GetOrder(ctx context.Context, orderNumber string) (int, error)
	GetOrders(ctx context.Context, userID int) ([]ordersrepo.Order, error)
	Timeout() time.Duration
}

func NewRepo(db *postgres.DB) database {
	return ordersrepo.NewOrder(db)
}

func GetOrdersHandler(repo database) func(w http.ResponseWriter, r *http.Request) {
	fn := func(w http.ResponseWriter, r *http.Request) {

		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), repo.Timeout())
		defer cancel()

		orders, err := repo.GetOrders(ctx, userID)
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(orders)
	}
	return fn
}

func PostOrdersHandler(repo database) func(w http.ResponseWriter, r *http.Request) {
	fn := func(w http.ResponseWriter, r *http.Request) {

		userID, err := strconv.Atoi(w.Header().Get("UID"))
		if err != nil {
			logger.Warnf("UID validate error: " + err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

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
			logger.Infof("goluhn validate error: " + err.Error() + " - " + responseString)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), repo.Timeout())
		defer cancel()

		orderUID, err := repo.GetOrder(ctx, responseString)
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

		err = repo.AddOrder(ctx, userID, responseString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
	return fn
}

func CheckOrders(FlagASAddr string, CheckOrdersTimeout time.Duration, db *postgres.DB) {

	ctx, cancel := context.WithTimeout(context.Background(), CheckOrdersTimeout)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			AwaitOrders, err := ordersrepo.GetAwaitOrders(ctx, db)
			if err != nil {
				logger.Warnf(err.Error())
				continue
			}
			if len(AwaitOrders) == 0 {
				continue
			}
			for _, order := range AwaitOrders {
				err = sendOrdersHandler(ctx, db, FlagASAddr, order)
				if err != nil {
					logger.Warnf(err.Error())
				}
			}
			continue
		}
	}
}

func sendOrdersHandler(ctx context.Context, db *postgres.DB, FlagASAddr string, o ordersrepo.Order) error {

	client := &http.Client{}
	url := fmt.Sprintf("%s/api/orders/%v", FlagASAddr, o.OrderNumber)

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

	orderUID, err := NewRepo(db).GetOrder(ctx, o.OrderNumber)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusNoContent {
		o.OrderStatus = "INVALID"
		ordersrepo.UpdateOrder(ctx, db, orderUID, o)
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

	ordersrepo.UpdateOrder(ctx, db, orderUID, o)
	return nil
}
