package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestUserCache(pool *pgxpool.Pool) *cache.UserCache {
	return cache.NewUserCache(pool, nil)
}

func TestUserFromContext_Present(t *testing.T) {
	u := &models.User{ID: uuid.New(), DisplayName: "Alice"}
	ctx := context.WithValue(context.Background(), middleware.ContextKeyUser, u)
	got := middleware.UserFromContext(ctx)
	if got == nil || got.DisplayName != "Alice" {
		t.Errorf("UserFromContext: got %v", got)
	}
}

func TestUserFromContext_Missing(t *testing.T) {
	if got := middleware.UserFromContext(context.Background()); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestAdminOnly_MissingUID(t *testing.T) {
	called := false
	handler := middleware.AdminOnly(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	// No ContextKeySupabaseUID in context.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
	if called {
		t.Error("next handler should not be called")
	}
}

func TestAdminOnly_NonAdminUser(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	// Seed a non-admin user.
	u := testutil.SeedUser(t, pool, "uid-admin-test", "NotAdmin", "notadmin@test.com")

	called := false
	userCache := newTestUserCache(pool)
	handler := middleware.AdminOnly(userCache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeySupabaseUID, u.SupabaseUID))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
	if called {
		t.Error("next handler should not be called for non-admin")
	}
}
