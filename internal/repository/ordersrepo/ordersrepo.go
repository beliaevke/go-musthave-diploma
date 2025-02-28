package ordersrepo

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/beliaevke/go-musthave-diploma/internal/db/postgres"
	"github.com/beliaevke/go-musthave-diploma/internal/logger"
	"github.com/beliaevke/go-musthave-diploma/internal/repository/queries"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Order struct {
	OrderNumber string    `db:"ordernumber" json:"number"`
	OrderStatus string    `db:"orderstatus" json:"status"`
	Accrual     float32   `db:"accrual" json:"accrual,omitempty"`
	UploadedAt  time.Time `db:"uploadedat" json:"uploaded_at"`
	db          *postgres.DB
}

func NewOrder(db *postgres.DB) *Order {
	return &Order{db: db}
}

func (o *Order) Timeout() time.Duration {
	return o.db.DefaultTimeout
}

func (o *Order) AddOrder(ctx context.Context, userID int, orderNumber string) error {
	tx, err := o.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint
	result := o.db.Pool.QueryRow(ctx, queries.GetOrderQueryRow, orderNumber)
	var val int
	switch err := result.Scan(&val); err {
	case pgx.ErrNoRows:
		_, err = o.db.Pool.Exec(ctx, queries.AddOrderInsert, userID, orderNumber, "NEW", time.Now())
		if err != nil {
			logger.Warnf("INSERT INTO Orders: " + err.Error())
			return err
		}
	case nil:
		err = errors.New("order already exists, uid: " + strconv.Itoa(val))
		if err != nil {
			logger.Warnf("INSERT INTO Orders: " + err.Error())
			return err
		}
	case err:
		logger.Warnf("Query AddOrder: " + err.Error())
		return err
	}
	return tx.Commit(ctx)
}

func (o *Order) GetOrder(ctx context.Context, orderNumber string) (int, error) {
	var val int
	result := o.db.Pool.QueryRow(ctx, queries.GetOrderQueryRow, orderNumber)
	switch err := result.Scan(&val); err {
	case pgx.ErrNoRows:
		return -1, nil
	case nil:
		return val, nil
	case err:
		logger.Warnf("Query GetOrder: " + err.Error())
		return -1, err
	}
	return val, nil
}

func (o *Order) GetOrders(ctx context.Context, userID int) ([]Order, error) {
	var val []Order
	result, err := o.db.Pool.Query(ctx, queries.GetOrdersQueryRow, userID)
	if err != nil {
		logger.Warnf("Query GetOrders: " + err.Error())
		return val, err
	}
	val, err = pgx.CollectRows(result, pgx.RowToStructByName[Order])
	if err != nil {
		logger.Warnf("CollectRows GetOrders: " + err.Error())
		return val, err
	}
	return val, nil
}

func GetAwaitOrders(ctx context.Context, db *postgres.DB) ([]Order, error) {
	var val []Order
	result, err := db.Pool.Query(ctx, queries.GetAwaitOrdersQueryRow)
	if err != nil {
		logger.Warnf("Query GetAwaitOrders: " + err.Error())
		return val, err
	}
	val, err = pgx.CollectRows(result, pgx.RowToStructByName[Order])
	if err != nil {
		logger.Warnf("CollectRows GetAwaitOrders: " + err.Error())
		return val, err
	}
	return val, nil
}

func UpdateOrder(ctx context.Context, db *postgres.DB, orderUID int, o Order) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint
	_, err = db.Pool.Exec(ctx, queries.UpdateOrderInsert, orderUID, o.OrderNumber, o.Accrual, time.Now())
	if err != nil {
		logger.Warnf("INSERT INTO UpdateOrder: " + err.Error())
		return err
	}
	_, err = db.Pool.Exec(ctx, queries.UpdateOrderQuery, o.OrderStatus, o.Accrual, time.Now(), orderUID, o.OrderNumber)
	if err != nil {
		logger.Warnf("UPDATE orders: " + err.Error())
		return err
	}
	_, err = db.Pool.Exec(ctx, queries.UpdateBalanceQuery, o.Accrual, orderUID)
	if err != nil {
		logger.Warnf("UPDATE usersbalance++: " + err.Error())
		return err
	}
	return tx.Commit(ctx)
}
