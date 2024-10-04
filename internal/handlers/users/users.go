package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/beliaevke/go-musthave-diploma/internal/db/postgres"
	"github.com/beliaevke/go-musthave-diploma/internal/logger"
	"github.com/beliaevke/go-musthave-diploma/internal/middleware/auth"
	"github.com/beliaevke/go-musthave-diploma/internal/repository/usersrepo"
	"github.com/beliaevke/go-musthave-diploma/internal/service"

	"github.com/avast/retry-go/v4"
	"github.com/golang-jwt/jwt/v4"
)

type database interface {
	CreateUser(ctx context.Context, u usersrepo.UserInfo) (int, error)
	GetUser(ctx context.Context, u usersrepo.UserInfo) (int, error)
	LoginUser(ctx context.Context, u usersrepo.UserInfo) (int, error)
	Timeout() time.Duration
}

const tokenExpiresAt = time.Second * 30 //time.Minute * 5 //

func authenticateUser(w http.ResponseWriter, userID int) error {
	// создаём случайный ключ
	key, err := service.GenerateRandom(16)
	if err != nil {
		return err
	}
	// создаём новый токен с алгоритмом подписи HS256 и утверждениями — Claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
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

func NewRepo(db *postgres.DB) database {
	return usersrepo.NewUser(db)
}

func UserRegisterHandler(repo database) func(w http.ResponseWriter, r *http.Request) {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			return
		}

		if n == 0 {
			return
		}

		var user usersrepo.UserInfo
		err = retry.Do(func() error {
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

		ctx, cancel := context.WithTimeout(r.Context(), repo.Timeout())
		defer cancel()

		userID, err := repo.GetUser(ctx, user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if userID != -1 {
			http.Error(w, "user already exists with this login", http.StatusConflict)
			return
		}
		userID, err = repo.CreateUser(ctx, user)
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
	return fn
}

func UserLoginHandler(repo database) func(w http.ResponseWriter, r *http.Request) {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			return
		}

		if n == 0 {
			return
		}

		var user usersrepo.UserInfo
		err = retry.Do(func() error {
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

		ctx, cancel := context.WithTimeout(r.Context(), repo.Timeout())
		defer cancel()

		userID, err := repo.LoginUser(ctx, user)
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
	return fn
}
