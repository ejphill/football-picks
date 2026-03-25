package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evan/football-picks/internal/api/middleware"
)

func TestETagReturns200WithHeaderOnFirstRequest(t *testing.T) {
	handler := middleware.ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"entries":[]}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/leaderboard/weekly", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("expected ETag header to be set")
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Error("expected Cache-Control header to be set")
	}
}

func TestETagReturns304WhenContentUnchanged(t *testing.T) {
	handler := middleware.ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"entries":[]}`))
	}))

	// First request — capture the ETag.
	req1 := httptest.NewRequest(http.MethodGet, "/leaderboard/weekly", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first request: no ETag header")
	}

	// Second request with matching If-None-Match — expect 304.
	req2 := httptest.NewRequest(http.MethodGet, "/leaderboard/weekly", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Errorf("status: got %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("expected empty body on 304, got %q", rec2.Body.String())
	}
}

func TestETagReturns200WhenContentChanges(t *testing.T) {
	body := `{"entries":[]}`
	handler := middleware.ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/leaderboard/weekly", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")

	// Content changes between requests.
	body = `{"entries":[{"user_id":"abc"}]}`

	req2 := httptest.NewRequest(http.MethodGet, "/leaderboard/weekly", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec2.Code)
	}
	if rec2.Header().Get("ETag") == etag {
		t.Error("expected new ETag when content changes")
	}
}

func TestETagPassesThroughNonOKGetResponse(t *testing.T) {
	// GET handler that returns 400 — ETag should not be set and body passes through.
	handler := middleware.ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
	if rec.Header().Get("ETag") != "" {
		t.Error("ETag should not be set for non-200 GET responses")
	}
}

func TestETagPassesThroughNonGET(t *testing.T) {
	called := false
	handler := middleware.ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/picks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called for POST")
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}
	if rec.Header().Get("ETag") != "" {
		t.Error("ETag should not be set for non-GET requests")
	}
}
