package usersrepo

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strconv"

	"musthave-diploma/internal/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

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

func (u *User) CreateUser(ctx context.Context, dbpool *pgxpool.Pool) (int, error) {
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return -1, err
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
			return -1, err
		}
		userID, err := u.GetUser(ctx, dbpool)
		if err != nil {
			logger.Warnf("CreateUser ID : " + err.Error())
			return userID, err
		}
		_, err = dbpool.Exec(ctx, `
			INSERT INTO public.usersbalance
			(userID, pointsSum, pointsLoss)
			VALUES
			($1, $2, $3);
		`, userID, 0, 0)
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

func (u *User) GetUser(ctx context.Context, dbpool *pgxpool.Pool) (int, error) {
	if u.UserLogin == "" || u.UserPassword == "" {
		err := errors.New("user or pass is empty")
		if err != nil {
			logger.Warnf("GetUser: " + err.Error())
			return -1, err
		}
	}
	result := dbpool.QueryRow(ctx, `
		SELECT users.userID
		FROM
			public.users
		WHERE
		users.userLogin=$1
	`, u.UserLogin)
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

func (u *User) LoginUser(ctx context.Context, dbpool *pgxpool.Pool) (int, error) {
	if u.UserLogin == "" || u.UserPassword == "" {
		err := errors.New("user or pass is empty")
		if err != nil {
			logger.Warnf("GetUser: " + err.Error())
			return -1, err
		}
	}
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
