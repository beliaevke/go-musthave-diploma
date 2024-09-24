package ordersrepo

import (
	"context"
	"errors"
	"strconv"
	"time"

	"musthave-diploma/internal/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Order struct {
	OrderNumber string    `db:"ordernumber" json:"number"`
	OrderStatus string    `db:"orderstatus" json:"status"`
	Accrual     float32   `db:"accrual" json:"accrual,omitempty"`
	UploadedAt  time.Time `db:"uploadedat" json:"uploaded_at"`
}

func NewOrder() *Order {
	return &Order{}
}

func (o *Order) AddOrder(ctx context.Context, dbpool *pgxpool.Pool, userID int, orderNumber string) error {
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint
	result := dbpool.QueryRow(ctx, `
		SELECT orders.userID
		FROM
			public.orders
		WHERE
		orders.ordernumber=$1
		`, orderNumber)
	var val int
	switch err := result.Scan(&val); err {
	case pgx.ErrNoRows:
		_, err = dbpool.Exec(ctx, `
			INSERT INTO public.orders
			(userID, ordernumber, orderstatus, uploadedat)
			VALUES
			($1, $2, $3, $4);
		`, userID, orderNumber, "NEW", time.Now())
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

func (o *Order) GetOrder(ctx context.Context, dbpool *pgxpool.Pool, orderNumber string) (int, error) {
	var val int
	result := dbpool.QueryRow(ctx, `
		SELECT orders.userID
		FROM
			public.orders
		WHERE
			orders.ordernumber=$1
	`, orderNumber)
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

func (o *Order) GetOrders(ctx context.Context, dbpool *pgxpool.Pool, userID int) ([]Order, error) {
	var val []Order
	result, err := dbpool.Query(ctx, `
		SELECT ordernumber, orderstatus, accrual, uploadedat
		FROM
			public.orders
		WHERE
			orders.userID=$1
		ORDER BY
			orders.uploadedat DESC
	`, userID)
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

func GetAwaitOrders(ctx context.Context, dbpool *pgxpool.Pool) ([]Order, error) {
	var val []Order
	result, err := dbpool.Query(ctx, `
		SELECT ordernumber, orderstatus, accrual, uploadedat
		FROM
			public.orders
		WHERE
			orders.orderstatus != 'INVALID' AND orders.orderstatus != 'PROCESSED'
		ORDER BY
			orders.uploadedat
	`)
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

func UpdateOrder(ctx context.Context, dbpool *pgxpool.Pool, orderUID int, o Order) error {
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint
	_, err = dbpool.Exec(ctx, `
		INSERT INTO public.ordersoperations
		(userID, orderNumber, pointsQuantity, processedAt)
		VALUES
		($1, $2, $3, $4)
		`, orderUID, o.OrderNumber, o.Accrual, time.Now())
	if err != nil {
		logger.Warnf("INSERT INTO UpdateOrder: " + err.Error())
		return err
	}
	_, err = dbpool.Exec(ctx, `
		UPDATE public.orders
		SET orderStatus=$1, accrual=$2, uploadedAt=$3
		WHERE userID=$4 AND orderNumber=$5;
	`, o.OrderStatus, o.Accrual, time.Now(), orderUID, o.OrderNumber)
	if err != nil {
		logger.Warnf("UPDATE orders: " + err.Error())
		return err
	}
	_, err = dbpool.Exec(ctx, `
		UPDATE public.usersbalance
		SET pointssum=$1
		WHERE userID=$2;
	`, o.Accrual, orderUID)
	if err != nil {
		logger.Warnf("UPDATE usersbalance++: " + err.Error())
		return err
	}
	return tx.Commit(ctx)
}
