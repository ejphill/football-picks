package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/api/middleware"
)

func TestTimeout_RequestCompletesBeforeDeadline(t *testing.T) {
	called := false
	handler := middleware.Timeout(500 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Context().Err() != nil {
			t.Error("context should not be cancelled within deadline")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestTimeout_ContextIsCancelledAfterDeadline(t *testing.T) {
	done := make(chan struct{})
	handler := middleware.Timeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait for the deadline to fire.
		select {
		case <-r.Context().Done():
			close(done)
		case <-time.After(500 * time.Millisecond):
			t.Error("context was not cancelled within expected time")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	select {
	case <-done:
		// Context was cancelled — expected.
	case <-time.After(time.Second):
		t.Error("timeout: context was never cancelled")
	}
}

func TestTimeout_WrapsNextHandler(t *testing.T) {
	// Ensure middleware passes the request through to the next handler.
	var capturedMethod string
	handler := middleware.Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedMethod != http.MethodDelete {
		t.Errorf("method: got %q, want DELETE", capturedMethod)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}
}
