package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllowAndCleanup(t *testing.T) {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    2,
		window:   time.Minute,
	}

	if !rl.Allow("203.0.113.10") {
		t.Fatal("expected first request to be allowed")
	}
	if !rl.Allow("203.0.113.10") {
		t.Fatal("expected second request to be allowed")
	}
	if rl.Allow("203.0.113.10") {
		t.Fatal("expected third request to be rate limited")
	}

	rl.requests["198.51.100.7"] = []time.Time{time.Now().Add(-2 * time.Minute)}
	rl.cleanup()
	if _, ok := rl.requests["198.51.100.7"]; ok {
		t.Fatal("expected expired request history to be removed")
	}
}

func TestRateLimitUsesForwardedFor(t *testing.T) {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    1,
		window:   time.Hour,
	}

	calls := 0
	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.1:1111"
	req1.Header.Set("X-Forwarded-For", "203.0.113.20")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusNoContent {
		t.Fatalf("expected first request to pass, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.2:2222"
	req2.Header.Set("X-Forwarded-For", "203.0.113.20")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be limited, got %d", rec2.Code)
	}

	if calls != 1 {
		t.Fatalf("expected wrapped handler to run once, ran %d times", calls)
	}
}
