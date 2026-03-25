package middleware

import (
	"context"
	"net/http"

	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/models"
)

// ContextKeyUser is exported so handler tests can inject a user directly.
const ContextKeyUser contextKey = "app_user"

// AdminOnly must run after Auth.
func AdminOnly(users *cache.UserCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid := SupabaseUIDFromContext(r.Context())
			if uid == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := users.Get(r.Context(), uid)
			if err != nil || !user.IsAdmin {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(ContextKeyUser).(*models.User)
	return u
}
