package usersrepo

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/beliaevke/go-musthave-diploma/internal/db/postgres"
	"github.com/beliaevke/go-musthave-diploma/internal/logger"
	"github.com/beliaevke/go-musthave-diploma/internal/repository/queries"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type User struct {
	db *postgres.DB
}

type UserInfo struct {
	UserID       int    `json:"id,omitempty"`
	UserLogin    string `json:"login"`
	UserPassword string `json:"password"`
}

func NewUser(db *postgres.DB) *User {
	return &User{
		db: db,
	}
}

func (ur *User) Timeout() time.Duration {
	return ur.db.DefaultTimeout
}

func (ur *User) CreateUser(ctx context.Context, u UserInfo) (int, error) {
	tx, err := ur.db.Pool.Begin(ctx)
	if err != nil {
		return -1, err
	}
	defer tx.Rollback(ctx) //nolint
	result := ur.db.Pool.QueryRow(ctx, queries.SelectUser, u.UserLogin)
	switch err := result.Scan(&u.UserLogin); err {
	case pgx.ErrNoRows:
		hash := md5.Sum([]byte(u.UserPassword))
		hashedPass := hex.EncodeToString(hash[:])
		_, err = ur.db.Pool.Exec(ctx, queries.CreateUserInsert, u.UserLogin, hashedPass)
		if err != nil {
			logger.Warnf("INSERT INTO Users: " + err.Error())
			return -1, err
		}
		userID, err := ur.GetUser(ctx, u)
		if err != nil {
			logger.Warnf("CreateUser ID : " + err.Error())
			return userID, err
		}
		_, err = ur.db.Pool.Exec(ctx, queries.CreateUserBalanceInsert, userID, 0, 0)
		if err != nil {
			logger.Warnf("INSERT INTO balance: " + err.Error())
			return userID, err
		}
		return userID, tx.Commit(ctx)
	case nil:
		err = errors.New("user already exists with this login")
		if err != nil {
			logger.Warnf("INSERT INTO Users: " + err.Error())
			return -1, err
		}
	case err:
		logger.Warnf("Query CreateUser: " + err.Error())
		return -1, err
	}
	return -1, tx.Commit(ctx)
}

func (ur *User) GetUser(ctx context.Context, u UserInfo) (int, error) {
	if u.UserLogin == "" || u.UserPassword == "" {
		err := errors.New("user or pass is empty")
		if err != nil {
			logger.Warnf("GetUser: " + err.Error())
			return -1, err
		}
	}
	result := ur.db.Pool.QueryRow(ctx, queries.SelectUser, u.UserLogin)
	switch err := result.Scan(&u.UserID); err {
	case pgx.ErrNoRows:
		return -1, nil
	case nil:
		return u.UserID, nil
	case err:
		var PassIsEmpty string
		if u.UserPassword == "" {
			PassIsEmpty = " -- PASS is empty"
		} else {
			PassIsEmpty = " -- PASS is not empty"
		}
		logger.Warnf("Query GetUser: " + err.Error() + " ID: " + strconv.Itoa(u.UserID) + " USER: " + u.UserLogin + PassIsEmpty)
		return -1, nil
	}
	return u.UserID, nil
}

func (ur *User) LoginUser(ctx context.Context, u UserInfo) (int, error) {
	if u.UserLogin == "" || u.UserPassword == "" {
		err := errors.New("user or pass is empty")
		if err != nil {
			logger.Warnf("GetUser: " + err.Error())
			return -1, err
		}
	}
	hash := md5.Sum([]byte(u.UserPassword))
	hashedPass := hex.EncodeToString(hash[:])
	result := ur.db.Pool.QueryRow(ctx, queries.SelectUserWithPass, u.UserLogin, hashedPass)
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
