package balancerepo

import (
	"context"
	"time"

	"musthave-diploma/internal/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

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
		logger.Warnf("UPDATE usersbalance--: " + err.Error())
		return err
	}
	return tx.Commit(ctx)
}

func (b *Balance) GetWithdrawals(ctx context.Context, dbpool *pgxpool.Pool, userID int) ([]Withdrawals, error) {
	var val []Withdrawals
	result, err := dbpool.Query(ctx, `
		SELECT orderNumber, -pointsQuantity as pointsQuantity, processedAt
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
