package postgres

import (
	"context"
	"crypto/md5"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"musthave-diploma/internal/logger"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/avast/retry-go/v4"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

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

type RetryAfterError struct {
	Config pgxpool.Config
}

func (err RetryAfterError) Error() string {
	return fmt.Sprintf(
		"Connection to %v error: %v",
		err.Config.ConnString(),
		err.Config.ConnConfig.OnPgError,
	)
}

type SomeOtherError struct {
	err        string
	retryAfter time.Duration
}

func (err SomeOtherError) Error() string {
	return err.err
}

func NewPSQL(user string, pass string, host string, port string, db string) Settings {
	return Settings{
		User:   user,
		Pass:   pass,
		Host:   host,
		Port:   port,
		DBName: db,
		ConnStr: fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
			host+":"+port, user, pass, db),
	}
}

func NewPSQLStr(connection string) *Settings {
	return &Settings{
		ConnStr: connection,
	}
}

func (s *Settings) Ping(ctx context.Context) error {
	err := retry.Do(func() error {
		dbpool, err := pgxpool.New(ctx, s.ConnStr)
		if err != nil {
			return err
		}
		defer dbpool.Close()
		err = dbpool.Ping(ctx)
		if err != nil {
			return err
		}
		return nil
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.Delay(time.Second),
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			switch e := err.(type) {
			case RetryAfterError:
				return 2 * time.Second
			case SomeOtherError:
				return e.retryAfter
			}
			//default is backoffdelay
			return retry.BackOffDelay(n, err, config)
		}),
		retry.Context(ctx),
	)
	return err
}

func (s *Settings) CreateUser(ctx context.Context, user User) error {
	dbpool, err := pgxpool.New(ctx, s.ConnStr)
	if err != nil {
		return err
	}
	defer dbpool.Close()
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
		`, user.UserLogin)
	switch err := result.Scan(&user.UserLogin); err {
	case pgx.ErrNoRows:
		hash := md5.Sum([]byte(user.UserPassword))
		hashedPass := hex.EncodeToString(hash[:])
		_, err = dbpool.Exec(ctx, `
			INSERT INTO public.users
			(userLogin, userPassword)
			VALUES
			($1, $2);
		`, user.UserLogin, hashedPass)
		if err != nil {
			logger.Warnf("INSERT INTO Users: " + err.Error())
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

func (s *Settings) GetUser(ctx context.Context, login string) (int, error) {
	var val int
	dbpool, err := pgxpool.New(ctx, s.ConnStr)
	if err != nil {
		return -1, err
	}
	defer dbpool.Close()
	result := dbpool.QueryRow(ctx, `
		SELECT users.userID
		FROM
			public.users
		WHERE
		users.userLogin=$1
	`, login)
	switch err := result.Scan(&val); err {
	case pgx.ErrNoRows:
		return -1, nil
	case nil:
		return val, nil
	case err:
		logger.Warnf("Query GetUser: " + err.Error())
		return -1, err
	}
	return val, nil
}

func (s *Settings) LoginUser(ctx context.Context, login string, pass string) (int, error) {
	var userID int
	dbpool, err := pgxpool.New(ctx, s.ConnStr)
	if err != nil {
		return -1, err
	}
	defer dbpool.Close()
	hash := md5.Sum([]byte(pass))
	hashedPass := hex.EncodeToString(hash[:])
	result := dbpool.QueryRow(ctx, `
		SELECT users.userID
		FROM
			public.users
		WHERE
		users.userLogin=$1 AND users.userPassword = $2
	`, login, hashedPass)
	switch err := result.Scan(&userID); err {
	case pgx.ErrNoRows:
		return -1, nil
	case nil:
		return userID, nil
	case err:
		logger.Warnf("Query LoginUser: " + err.Error())
		return -1, err
	}
	return userID, nil
}

func (s *Settings) AddOrder(ctx context.Context, userID int, orderNumber string) error {
	dbpool, err := pgxpool.New(ctx, s.ConnStr)
	if err != nil {
		return err
	}
	defer dbpool.Close()
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
		orders.orderNumber=$1
		`, orderNumber)
	var orderStatus string
	switch err := result.Scan(&orderStatus); err {
	case pgx.ErrNoRows:
		_, err = dbpool.Exec(ctx, `
			INSERT INTO public.orders
			(userID, orderNumber, orderStatus, uploadedAt)
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

func (s *Settings) GetOrder(ctx context.Context, orderNumber string) (int, error) {
	var val int
	dbpool, err := pgxpool.New(ctx, s.ConnStr)
	if err != nil {
		return -1, err
	}
	defer dbpool.Close()
	result := dbpool.QueryRow(ctx, `
		SELECT orders.userID
		FROM
			public.orders
		WHERE
		orders.orderNumber=$1
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

func SetDB(ctx context.Context, databaseURI string) {
	db, err := sql.Open("pgx", databaseURI)
	if err != nil {
		logger.Warnf("sql.Open(): " + err.Error())
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warnf("goose: failed to close DB: " + err.Error())
		}
	}()
	goose.SetBaseFS(embedMigrations)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		logger.Warnf("goose up: run failed  " + err.Error())
	}
}
