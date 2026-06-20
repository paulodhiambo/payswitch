package middleware

import (
	"context"
	"net/http"
	"time"
)

type contextKey string

const IdempotencyCtxKey contextKey = "idempotency_key"

type Cache interface {
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
}

func Idempotency(c Cache, ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idemKey := r.Header.Get("Idempotency-Key")
			if idemKey == "" {
				http.Error(w, "Idempotency-Key header required", http.StatusBadRequest)
				return
			}
			key := "idem:" + idemKey
			ok, err := c.SetNX(r.Context(), key, "processing", ttl)
			if err != nil {
				http.Error(w, "cache error", http.StatusInternalServerError)
				return
			}
			if !ok {
				http.Error(w, "duplicate request", http.StatusConflict)
				return
			}
			ctx := context.WithValue(r.Context(), IdempotencyCtxKey, idemKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
