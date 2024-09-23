package repository

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"time"

	"musthave-diploma/internal/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Settings struct {
	User    string
	Pass    string
	Host    string
	Port    string
	DBName  string
	ConnStr string
}

type User struct {
	UserID       int    `json:"id,omitempty"`
	UserLogin    string `json:"login"`
	UserPassword string `json:"password"`
}

func NewUser(id int, login string, pass string) *User {
	return &User{
		UserID:       id,
		UserLogin:    login,
		UserPassword: pass,
	}
}

type Order struct {
	OrderNumber string    `db:"ordernumber" json:"number"`
	OrderStatus string    `db:"orderstatus" json:"status"`
	Accrual     float32   `db:"accrual" json:"accrual,omitempty"`
	UploadedAt  time.Time `db:"uploadedat" json:"uploaded_at"`
}

func NewOrder() *Order {
	return &Order{}
}

type Balance struct {
	PointsSum  float32 `db:"pointssum" json:"current"`
	PointsLoss float32 `db:"pointsloss" json:"withdrawn"`
}

func NewBalance() *Balance {
	return &Balance{}
}

type Withdraw struct {
	OrderNumber string  `json:"order"`
	Sum         float32 `json:"sum"`
}

type Withdrawals struct {
	OrderNumber    string    `db:"ordernumber" json:"order"`
	PointsQuantity float32   `db:"pointsquantity" json:"sum"`
	ProcessedAt    time.Time `db:"processedat" json:"processed_at"`
}

func (u *User) CreateUser(ctx context.Context, dbpool *pgxpool.Pool) error {
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint
	result := dbpool.QueryRow(ctx, `
		SELECT users.userID
		FROM
			public.users
		WHERE
		users.userLogin=$1
		`, u.UserLogin)
	switch err := result.Scan(&u.UserLogin); err {
	case pgx.ErrNoRows:
		hash := md5.Sum([]byte(u.UserPassword))
		hashedPass := hex.EncodeToString(hash[:])
		_, err = dbpool.Exec(ctx, `
			INSERT INTO public.users
			(userLogin, userPassword)
			VALUES
			($1, $2);
		`, u.UserLogin, hashedPass)
		if err != nil {
			logger.Warnf("INSERT INTO Users: " + err.Error())
			return err
		}
		userID, err := u.GetUser(ctx, dbpool)
		if err != nil {
			logger.Warnf("CreateUser ID : " + err.Error())
			return err
		}
		_, err = dbpool.Exec(ctx, `
			INSERT INTO public.usersbalance
			(userID, pointsSum, pointsLoss)
			VALUES
			($1, $2, $3);
		`, userID, 100, 23.5)
		if err != nil {
			logger.Warnf("INSERT INTO balance: " + err.Error())
			return err
		}
	case nil:
		err = errors.New("user already exists with this login")
		if err != nil {
			logger.Warnf("INSERT INTO Users: " + err.Error())
			return err
		}
	case err:
		logger.Warnf("Query CreateUser: " + err.Error())
		return err
	}
	return tx.Commit(ctx)
}

func (u *User) GetUser(ctx context.Context, dbpool *pgxpool.Pool) (int, error) {
	result := dbpool.QueryRow(ctx, `
		SELECT users.userID
		FROM
			public.users
		WHERE
		users.userLogin=$1
	`, u.UserLogin)
	switch err := result.Scan(u.UserID); err {
	case pgx.ErrNoRows:
		return -1, nil
	case nil:
		return u.UserID, nil
	case err:
		logger.Warnf("Query GetUser: " + err.Error())
		return -1, nil
	}
	return u.UserID, nil
}

func (u *User) LoginUser(ctx context.Context, dbpool *pgxpool.Pool) (int, error) {
	hash := md5.Sum([]byte(u.UserPassword))
	hashedPass := hex.EncodeToString(hash[:])
	result := dbpool.QueryRow(ctx, `
		SELECT users.userID
		FROM
			public.users
		WHERE
		users.userLogin=$1 AND users.userPassword = $2
	`, u.UserLogin, hashedPass)
	switch err := result.Scan(&u.UserID); err {
	case pgx.ErrNoRows:
		return -1, nil
	case nil:
		return u.UserID, nil
	case err:
		logger.Warnf("Query LoginUser: " + err.Error())
		return -1, nil
	}
	return u.UserID, nil
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
	var orderStatus string
	switch err := result.Scan(&orderStatus); err {
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
		err = errors.New("order already exists")
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

func (b *Balance) GetBalance(ctx context.Context, dbpool *pgxpool.Pool, userID int) (Balance, error) {
	var val Balance
	result := dbpool.QueryRow(ctx, `
		SELECT pointsSum, pointsLoss
		FROM
			public.usersbalance
		WHERE
			usersbalance.userID=$1
	`, userID)
	switch err := result.Scan(&val.PointsSum, &val.PointsLoss); err {
	case pgx.ErrNoRows:
		return val, nil
	case nil:
		return val, nil
	case err:
		logger.Warnf("Query GetBalance: " + err.Error())
		return val, err
	}
	return val, nil
}

func (b *Balance) BalanceWithdraw(ctx context.Context, dbpool *pgxpool.Pool, userID int, userBalance Balance, withdraw Withdraw) error {
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
		`, userID, withdraw.OrderNumber, -withdraw.Sum, time.Now())
	if err != nil {
		logger.Warnf("INSERT INTO OrdersOperations: " + err.Error())
		return err
	}

	_, err = dbpool.Exec(ctx, `
		UPDATE public.usersbalance
		SET pointssum=$1, pointsloss=$2
		WHERE userID=$3;
	`, userBalance.PointsSum-withdraw.Sum, userBalance.PointsLoss+withdraw.Sum, userID)
	if err != nil {
		logger.Warnf("UPDATE usersbalance: " + err.Error())
		return err
	}
	return tx.Commit(ctx)
}

func (b *Balance) GetWithdrawals(ctx context.Context, dbpool *pgxpool.Pool, userID int) ([]Withdrawals, error) {
	var val []Withdrawals
	result, err := dbpool.Query(ctx, `
		SELECT orderNumber, pointsQuantity, processedAt
		FROM
			public.ordersoperations
		WHERE
			ordersoperations.userID=$1 AND ordersoperations.pointsQuantity < 0
		ORDER BY
			ordersoperations.processedAt DESC
	`, userID)
	if err != nil {
		logger.Warnf("Query GetWithdrawals: " + err.Error())
		return val, err
	}
	val, err = pgx.CollectRows(result, pgx.RowToStructByName[Withdrawals])
	if err != nil {
		logger.Warnf("CollectRows GetWithdrawals: " + err.Error())
		return val, err
	}
	return val, nil
}
