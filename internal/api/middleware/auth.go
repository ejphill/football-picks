package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	ContextKeySupabaseUID contextKey = "supabase_uid"
)

func NewJWKS(ctx context.Context, supabaseURL string) (keyfunc.Keyfunc, error) {
	jwksURL := strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"

	k, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS from %s: %w", jwksURL, err)
	}
	return k, nil
}

// Auth validates the ES256 JWT and injects the subject (Supabase UID) into context.
func Auth(jwks keyfunc.Keyfunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				http.Error(w, "missing authorization token", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenStr, jwks.Keyfunc,
				jwt.WithValidMethods([]string{"ES256"}),
				jwt.WithLeeway(5*time.Second),
			)
			if err != nil || !token.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "invalid token claims", http.StatusUnauthorized)
				return
			}

			sub, ok := claims["sub"].(string)
			if !ok || sub == "" {
				http.Error(w, "missing subject in token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeySupabaseUID, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

func SupabaseUIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeySupabaseUID).(string)
	return v
}
