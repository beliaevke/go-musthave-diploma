package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"musthave-diploma/internal/db/postgres"
	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/middleware/auth"
	"musthave-diploma/internal/repository/usersrepo"
	"musthave-diploma/internal/service"

	"github.com/avast/retry-go/v4"
	"github.com/golang-jwt/jwt/v4"
)

type database interface {
	CreateUser(ctx context.Context, db *postgres.DB) (int, error)
	GetUser(ctx context.Context, db *postgres.DB) (int, error)
	LoginUser(ctx context.Context, db *postgres.DB) (int, error)
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

func newRepo(id int, login string, pass string) database {
	return usersrepo.NewUser(id, login, pass)
}

func UserRegisterHandler(db *postgres.DB) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			return
		}

		if r.Method != http.MethodPost || n == 0 {
			return
		}

		var user usersrepo.User
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

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		repo := newRepo(user.UserID, user.UserLogin, user.UserPassword)

		userID, err := repo.GetUser(ctx, db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if userID != -1 {
			http.Error(w, "user already exists with this login", http.StatusConflict)
			return
		}
		userID, err = user.CreateUser(ctx, db)
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
	return http.HandlerFunc(fn)
}

func UserLoginHandler(db *postgres.DB) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			return
		}

		if r.Method != http.MethodPost || n == 0 {
			return
		}

		var user usersrepo.User
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

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		repo := newRepo(user.UserID, user.UserLogin, user.UserPassword)

		userID, err := repo.LoginUser(ctx, db)
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
	return http.HandlerFunc(fn)
}
