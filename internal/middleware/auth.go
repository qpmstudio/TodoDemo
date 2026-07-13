package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// Auth is middleware that validates JWT from httpOnly cookie and injects user info into context.
func Auth(cfg *config.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("jwt")
			if err != nil {
				writeUnauthorized(w, "JWT cookie missing")
				return
			}

			token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.JWTSecret), nil
			})
			if err != nil || !token.Valid {
				writeUnauthorized(w, "JWT is invalid or expired")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				writeUnauthorized(w, "Invalid JWT claims")
				return
			}

			userID, _ := claims["sub"].(string)
			userLogin, _ := claims["login"].(string)

			if userID == "" {
				writeUnauthorized(w, "JWT missing subject")
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserLoginKey, userLogin)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	body := `{"data":null,"error":{"code":"UNAUTHORIZED","message":"` + escapeJSON(msg) + `"}}`
	w.Write([]byte(body))
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
