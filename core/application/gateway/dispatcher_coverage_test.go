package gateway_test

import (
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// TestDispatcherCreation verifies dispatcher creation.
func TestDispatcherCreation(t *testing.T) {
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/ping"},
	})
	transport := &mockTransport{}
	jwt := &mockJWT{}
	schema := &mockSchema{}

	d := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:   2 * time.Second,
	})

	if d == nil {
		t.Fatal("expected dispatcher, got nil")
	}
}

// TestSecurityHeaders verifies headers are set correctly.
func TestSecurityHeaders_Coverage(t *testing.T) {
	headers := apgw.SecurityHeaders()

	expectedHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Content-Security-Policy",
	}

	for _, h := range expectedHeaders {
		if _, ok := headers[h]; !ok {
			t.Errorf("missing header: %s", h)
		}
	}
}

// TestRateLimiter_BasicCoverage verifies basic rate limiting.
func TestRateLimiter_BasicCoverage(t *testing.T) {
	limiter := apgw.NewRateLimiter(2, 2, time.Minute)

	// Should allow first 2 requests per IP
	if !limiter.AllowIP("10.0.0.1") {
		t.Error("first request should be allowed")
	}
	if !limiter.AllowIP("10.0.0.1") {
		t.Error("second request should be allowed")
	}
	if limiter.AllowIP("10.0.0.1") {
		t.Error("third request should be denied")
	}

	// Different IP should be allowed
	if !limiter.AllowIP("10.0.0.2") {
		t.Error("different IP should be allowed")
	}
}

// TestRateLimiter_TokenCoverage verifies token-based rate limiting.
func TestRateLimiter_TokenCoverage(t *testing.T) {
	limiter := apgw.NewRateLimiter(10, 1, time.Minute)

	// Should allow first request with token
	if !limiter.AllowToken("token-123") {
		t.Error("first token request should be allowed")
	}
	if limiter.AllowToken("token-123") {
		t.Error("second request with same token should be denied")
	}

	// Different token should be allowed
	if !limiter.AllowToken("token-456") {
		t.Error("different token should be allowed")
	}
}
