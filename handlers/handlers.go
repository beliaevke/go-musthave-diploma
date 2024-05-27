package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"musthave-diploma/internal/logger"
	"musthave-diploma/internal/postgres"
	"musthave-diploma/internal/service"

	"github.com/ShiraazMoollatjie/goluhn"
	"github.com/avast/retry-go/v4"
	"github.com/golang-jwt/jwt/v4"
)

type database interface {
	Ping(ctx context.Context) error
	GetUser(ctx context.Context, login string) (int, error)
	CreateUser(ctx context.Context, user postgres.User) error
	LoginUser(ctx context.Context, login string, pass string) (int, error)
	AddOrder(ctx context.Context, userID int, orderNumber string) error
	GetOrder(ctx context.Context, orderNumber string) (int, error)
}

type Claims struct {
	jwt.RegisteredClaims
	UserID int
}

type authenticationResponseWriter struct {
	http.ResponseWriter // встраиваем оригинальный http.ResponseWriter
	userID              int
}

const TOKEN_EXP = time.Minute * 5 //time.Second * 15

func WithAuthentication(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		st, err := r.Cookie("session_token")
		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sk, err := r.Cookie("session_key")
		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sessionToken, err := url.QueryUnescape(st.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		sessionKey, err := url.QueryUnescape(sk.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(sessionToken, claims,
			func(t *jwt.Token) (interface{}, error) {
				return []byte(sessionKey), nil
			})
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		if !token.Valid {
			http.Error(w, "token is not valid", http.StatusUnauthorized)
			return
		}

		aw := authenticationResponseWriter{
			ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
			userID:         claims.UserID,
		}

		aw.Header().Set("UID", strconv.Itoa(aw.userID))

		h.ServeHTTP(&aw, r)
	}
	return http.HandlerFunc(fn)
}

func authenticateUser(w http.ResponseWriter, userID int) error {
	// создаём случайный ключ
	key, err := service.GenerateRandom(16)
	if err != nil {
		return err
	}
	// создаём новый токен с алгоритмом подписи HS256 и утверждениями — Claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			// когда создан токен
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TOKEN_EXP)),
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
		Expires: time.Now().Add(TOKEN_EXP),
	})
	http.SetCookie(w, &http.Cookie{
		Name:    "session_key",
		Value:   url.QueryEscape(string(key)),
		Expires: time.Now().Add(TOKEN_EXP),
	})
	return nil
}

func setDB(databaseURI string) (database, error) {
	if databaseURI == "" {
		logger.Warnf("database URI is empty")
		return nil, errors.New("database URI is empty")
	}
	return postgres.NewPSQLStr(databaseURI), nil
}

func UserRegisterHandler(databaseURI string) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		db, err := setDB(databaseURI)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			return
		}
		if r.Method == http.MethodPost && n != 0 {
			var user postgres.User
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
			userID, err := db.GetUser(ctx, user.UserLogin)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if userID == -1 {
				http.Error(w, "user already exists with this login", http.StatusConflict)
				return
			}
			err = db.CreateUser(ctx, user)
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

func UserLoginHandler(databaseURI string) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		db, err := setDB(databaseURI)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			return
		}
		if r.Method == http.MethodPost && n != 0 {
			var user postgres.User
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
			userID, err := db.LoginUser(ctx, user.UserLogin, user.UserPassword)
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

func GetOrdersHandler(databaseURI string) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		db, err := setDB(databaseURI)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		// читаем тело запроса
		n, err := buf.ReadFrom(r.Body)
		if err != nil || n == 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodPost && n != 0 {
			responseString := buf.String()
			err = goluhn.Validate(responseString)
			if err != nil {
				logger.Warnf("goluhn validate error: " + err.Error())
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}

			userID, err := strconv.Atoi(w.Header().Get("UID"))
			if err != nil {
				logger.Warnf("UID validate error: " + err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			orderUID, err := db.GetOrder(ctx, responseString)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else if userID == orderUID {
				w.WriteHeader(http.StatusOK)
				return
			} else if userID != orderUID && orderUID != -1 {
				http.Error(w, "order already exists with another user", http.StatusConflict)
				return
			}

			err = db.AddOrder(ctx, userID, responseString)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusAccepted)
		}
	}
	return http.HandlerFunc(fn)
}
