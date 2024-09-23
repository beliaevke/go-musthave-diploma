package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/middleware/authentication"
	"musthave-diploma/internal/repository"
	"musthave-diploma/internal/service"

	"github.com/avast/retry-go/v4"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v5/pgxpool"
)

type database interface {
	CreateUser(ctx context.Context, dbpool *pgxpool.Pool) error
	GetUser(ctx context.Context, dbpool *pgxpool.Pool) (int, error)
	LoginUser(ctx context.Context, dbpool *pgxpool.Pool) (int, error)
}

const tokenExpiresAt = time.Minute * 5 //time.Second * 15

func authenticateUser(w http.ResponseWriter, userID int) error {
	// создаём случайный ключ
	key, err := service.GenerateRandom(16)
	if err != nil {
		return err
	}
	// создаём новый токен с алгоритмом подписи HS256 и утверждениями — Claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, authentication.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			// когда создан токен
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpiresAt)),
		},
		// собственное утверждение
		UserID: userID,
	})
	// создаём строку токена
	tokenString, err := token.SignedString([]byte(key))
	if err != nil {
		return err
	}
	// устанавливаем куки
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   url.QueryEscape(tokenString),
		Expires: time.Now().Add(tokenExpiresAt),
	})
	http.SetCookie(w, &http.Cookie{
		Name:    "session_key",
		Value:   url.QueryEscape(string(key)),
		Expires: time.Now().Add(tokenExpiresAt),
	})
	return nil
}

func initDB(id int, login string, pass string) database {
	return repository.NewUser(id, login, pass)
}

func UserRegisterHandler(dbpool *pgxpool.Pool) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			return
		}
		if r.Method == http.MethodPost && n != 0 {
			var user repository.User
			err := retry.Do(func() error {
				if err = json.Unmarshal(buf.Bytes(), &user); err != nil {
					return err
				}
				return nil
			},
				retry.Attempts(3),
				retry.Delay(1000*time.Millisecond),
			)
			if err != nil {
				logger.Warnf("JSON error: " + err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			db := initDB(user.UserID, user.UserLogin, user.UserPassword)

			userID, err := db.GetUser(ctx, dbpool)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if userID != -1 {
				http.Error(w, "user already exists with this login", http.StatusConflict)
				return
			}
			err = user.CreateUser(ctx, dbpool)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = authenticateUser(w, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}
	return http.HandlerFunc(fn)
}

func UserLoginHandler(dbpool *pgxpool.Pool) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			return
		}
		if r.Method == http.MethodPost && n != 0 {
			var user repository.User
			err := retry.Do(func() error {
				// десериализуем JSON в Visitor
				if err = json.Unmarshal(buf.Bytes(), &user); err != nil {
					return err
				}
				return nil
			},
				retry.Attempts(3),
				retry.Delay(1000*time.Millisecond),
			)
			if err != nil {
				logger.Warnf("JSON error: " + err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			db := initDB(user.UserID, user.UserLogin, user.UserPassword)

			userID, err := db.LoginUser(ctx, dbpool)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if userID == -1 {
				http.Error(w, "incorrect login or password", http.StatusUnauthorized)
				return
			}
			err = authenticateUser(w, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}
	return http.HandlerFunc(fn)
}
