package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/api/middleware"
	"golang.org/x/time/rate"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimiterAllowsBurst(t *testing.T) {
	// Burst of 3: first three requests from the same IP should succeed.
	limiter := middleware.NewRateLimiter(rate.Every(1000), 3)
	handler := limiter(okHandler())

	for i := range 3 {
		req := httptest.NewRequest(http.MethodPost, "/picks", nil)
		req.RemoteAddr = "1.2.3.4:9999"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}
}

func TestRateLimiterBlocksAfterBurstExhausted(t *testing.T) {
	// Burst of 2; third request in the same instant should be blocked.
	limiter := middleware.NewRateLimiter(rate.Every(time.Hour), 2)
	handler := limiter(okHandler())

	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/picks", nil)
		req.RemoteAddr = "5.6.7.8:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodPost, "/picks", nil)
	req.RemoteAddr = "5.6.7.8:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status: got %d, want 429", rec.Code)
	}
}

func TestRateLimiterTracksIPsSeparately(t *testing.T) {
	// Two IPs share the same limiter instance but each has its own bucket.
	limiter := middleware.NewRateLimiter(rate.Every(time.Hour), 1)
	handler := limiter(okHandler())

	for _, ip := range []string{"10.0.0.1:1", "10.0.0.2:1"} {
		req := httptest.NewRequest(http.MethodPost, "/picks", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("IP %s: got %d, want 200", ip, rec.Code)
		}
	}
}
