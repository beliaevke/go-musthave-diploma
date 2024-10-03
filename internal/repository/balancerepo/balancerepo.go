package balancerepo

import (
	"context"
	"time"

	"musthave-diploma/internal/db/postgres"
	"musthave-diploma/internal/logger"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Balance struct {
	PointsSum  float32 `db:"pointssum" json:"current"`
	PointsLoss float32 `db:"pointsloss" json:"withdrawn"`
	db         *postgres.DB
}

func NewBalance(db *postgres.DB) *Balance {
	return &Balance{db: db}
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

func (b *Balance) Timeout() time.Duration {
	return b.db.DefaultTimeout
}

func (b *Balance) GetBalance(ctx context.Context, userID int) (Balance, error) {
	var val Balance
	result := b.db.Pool.QueryRow(ctx, GetBalanceQueryRow(), userID)
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

func (b *Balance) BalanceWithdraw(ctx context.Context, userID int, userBalance Balance, withdraw Withdraw) error {
	tx, err := b.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint
	_, err = b.db.Pool.Exec(ctx, BalanceWithdrawInsert(), userID, withdraw.OrderNumber, -withdraw.Sum, time.Now())
	if err != nil {
		logger.Warnf("INSERT INTO OrdersOperations: " + err.Error())
		return err
	}

	_, err = b.db.Pool.Exec(ctx, BalanceWithdrawUpdate(), userBalance.PointsSum-withdraw.Sum, userBalance.PointsLoss+withdraw.Sum, userID)
	if err != nil {
		logger.Warnf("UPDATE usersbalance--: " + err.Error())
		return err
	}
	return tx.Commit(ctx)
}

func (b *Balance) GetWithdrawals(ctx context.Context, userID int) ([]Withdrawals, error) {
	var val []Withdrawals
	result, err := b.db.Pool.Query(ctx, GetWithdrawalsQuery(), userID)
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

////////////////////////////////////////
// queries

func GetBalanceQueryRow() string {
	return `
		SELECT pointsSum, pointsLoss
		FROM
			public.usersbalance
		WHERE
			usersbalance.userID=$1
	`
}

func BalanceWithdrawInsert() string {
	return `
		INSERT INTO public.ordersoperations
		(userID, orderNumber, pointsQuantity, processedAt)
		VALUES
		($1, $2, $3, $4)
	`
}

func BalanceWithdrawUpdate() string {
	return `
		UPDATE public.usersbalance
		SET pointssum=$1, pointsloss=$2
		WHERE userID=$3;
	`
}

func GetWithdrawalsQuery() string {
	return `
		SELECT orderNumber, -pointsQuantity as pointsQuantity, processedAt
		FROM
			public.ordersoperations
		WHERE
			ordersoperations.userID=$1 AND ordersoperations.pointsQuantity < 0
		ORDER BY
			ordersoperations.processedAt DESC
	`
}
