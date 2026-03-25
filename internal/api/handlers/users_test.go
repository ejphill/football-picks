package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evan/football-picks/internal/api/handlers"
	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRegister(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("creates new user", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doRegister(t, pool, "uid-reg-1", "Alice", "alice@test.com")
		if rr.Code != http.StatusCreated {
			t.Errorf("status: got %d, want 201 — body: %s", rr.Code, rr.Body.String())
		}
		var u models.User
		if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if u.DisplayName != "Alice" {
			t.Errorf("display_name: got %q, want Alice", u.DisplayName)
		}
	})

	t.Run("idempotent — returns existing user on re-register", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		testutil.SeedUser(t, pool, "uid-reg-2", "Bob", "bob@test.com")
		rr := doRegister(t, pool, "uid-reg-2", "Bob", "bob@test.com")
		if rr.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200 — body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("rejects missing display_name", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doRegister(t, pool, "uid-reg-3", "", "no-name@test.com")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("rejects missing email", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doRegisterRaw(t, pool, "uid-reg-4", map[string]string{"display_name": "NoEmail"})
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("rejects missing uid in context", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		// Empty UID — no context value.
		rr := doRegisterWithUID(t, pool, "", "Alice", "alice@test.com")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rr.Code)
		}
	})

	t.Run("400 for invalid json body", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		uh := handlers.NewUsersHandler(pool, cache.NewUserCache(pool, nil))
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte("not json")))
		r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, "uid-reg-inv"))
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router := chi.NewRouter()
		router.Post("/api/v1/auth/register", uh.Register)
		router.ServeHTTP(rr, r)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

func TestGetMe(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns user", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		u := testutil.SeedUser(t, pool, "uid-gm-1", "GetMe", "getme@test.com")

		rr := doGetMe(t, pool, u.SupabaseUID)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var got models.User
		json.Unmarshal(rr.Body.Bytes(), &got)
		if got.ID != u.ID {
			t.Errorf("user ID mismatch")
		}
	})

	t.Run("404 when user does not exist", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doGetMe(t, pool, "uid-nonexistent")
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})
}

func TestUpdateMe(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("updates display_name", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		u := testutil.SeedUser(t, pool, "uid-um-1", "Original", "orig@test.com")

		newName := "Updated"
		rr := doUpdateMe(t, pool, u.SupabaseUID, map[string]interface{}{"display_name": newName})
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var got models.User
		json.Unmarshal(rr.Body.Bytes(), &got)
		if got.DisplayName != newName {
			t.Errorf("display_name: got %q, want %q", got.DisplayName, newName)
		}
	})

	t.Run("updates notify_email", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		u := testutil.SeedUser(t, pool, "uid-um-2", "Notify", "notify@test.com")

		rr := doUpdateMe(t, pool, u.SupabaseUID, map[string]interface{}{"notify_email": true})
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var got models.User
		json.Unmarshal(rr.Body.Bytes(), &got)
		if !got.NotifyEmail {
			t.Error("notify_email should be true after update")
		}
	})

	t.Run("404 when user does not exist", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doUpdateMe(t, pool, "uid-nonexistent", map[string]interface{}{"display_name": "X"})
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})

	t.Run("400 for invalid json body", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		u := testutil.SeedUser(t, pool, "uid-um-inv", "InvUser", "invuser@test.com")
		uh := handlers.NewUsersHandler(pool, cache.NewUserCache(pool, nil))
		r := httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", bytes.NewReader([]byte("not json")))
		r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, u.SupabaseUID))
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		uh.UpdateMe(rr, r)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

// helpers

func doRegister(t *testing.T, pool *pgxpool.Pool, uid, displayName, email string) *httptest.ResponseRecorder {
	t.Helper()
	return doRegisterRaw(t, pool, uid, map[string]string{"display_name": displayName, "email": email})
}

func doRegisterWithUID(t *testing.T, pool *pgxpool.Pool, uid, displayName, email string) *httptest.ResponseRecorder {
	t.Helper()
	uh := handlers.NewUsersHandler(pool, cache.NewUserCache(pool, nil))
	body, _ := json.Marshal(map[string]string{"display_name": displayName, "email": email})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	if uid != "" {
		r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, uid))
	}
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Post("/api/v1/auth/register", uh.Register)
	router.ServeHTTP(rr, r)
	return rr
}

func doRegisterRaw(t *testing.T, pool *pgxpool.Pool, uid string, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	uh := handlers.NewUsersHandler(pool, cache.NewUserCache(pool, nil))
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, uid))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Post("/api/v1/auth/register", uh.Register)
	router.ServeHTTP(rr, r)
	return rr
}

func doGetMe(t *testing.T, pool *pgxpool.Pool, uid string) *httptest.ResponseRecorder {
	t.Helper()
	uh := handlers.NewUsersHandler(pool, cache.NewUserCache(pool, nil))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, uid))
	rr := httptest.NewRecorder()
	uh.GetMe(rr, r)
	return rr
}

func doUpdateMe(t *testing.T, pool *pgxpool.Pool, uid string, payload map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	uh := handlers.NewUsersHandler(pool, cache.NewUserCache(pool, nil))
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", bytes.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, uid))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	uh.UpdateMe(rr, r)
	return rr
}
