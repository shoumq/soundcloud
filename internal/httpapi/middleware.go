package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
		})
	}
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if isAllowedOrigin(origin, allowed) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin string, allowed map[string]struct{}) bool {
	if origin == "" {
		return false
	}

	if _, ok := allowed["*"]; ok {
		return true
	}

	if _, ok := allowed[origin]; ok {
		return true
	}

	return strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:")
}

func authMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sub, ok := bearerSubject(r, jwtSecret)
			if !ok {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			next.ServeHTTP(w, withUserID(r, sub))
		})
	}
}

func optionalAuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sub, ok := bearerSubject(r, jwtSecret)
			if ok {
				next.ServeHTTP(w, withUserID(r, sub))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerSubject(r *http.Request, jwtSecret string) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}

	tokenString := strings.TrimPrefix(header, "Bearer ")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return "", false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", false
	}

	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return "", false
	}

	return sub, true
}
