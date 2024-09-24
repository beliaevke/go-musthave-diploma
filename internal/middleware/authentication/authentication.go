package authentication

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID int
}

type authenticationResponseWriter struct {
	http.ResponseWriter // встраиваем оригинальный http.ResponseWriter
	userID              int
}

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

		if claims.UserID == -1 {
			http.Error(w, "UserID is incorrect", http.StatusUnauthorized)
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
