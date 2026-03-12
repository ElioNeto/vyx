package gateway_test

import (
	"fmt"
	"testing"
	"time"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
)

func TestRateLimiter_AllowIP_UnderLimit(t *testing.T) {
	rl := apgw.NewRateLimiter(5, 100, time.Minute)
	for i := 0; i < 5; i++ {
		if !rl.AllowIP("1.2.3.4:1234") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_AllowIP_ExceedsLimit(t *testing.T) {
	rl := apgw.NewRateLimiter(3, 100, time.Minute)
	for i := 0; i < 3; i++ {
		rl.AllowIP("10.0.0.1:9999")
	}
	if rl.AllowIP("10.0.0.1:9999") {
		t.Error("4th request should be denied")
	}
}

func TestRateLimiter_AllowToken_ExceedsLimit(t *testing.T) {
	rl := apgw.NewRateLimiter(100, 2, time.Minute)
	tok := "Bearer eyJhbGciOiJIUzI1NiJ9"
	rl.AllowToken(tok)
	rl.AllowToken(tok)
	if rl.AllowToken(tok) {
		t.Error("3rd token request should be denied")
	}
}

func TestRateLimiter_AllowToken_EmptyToken_AlwaysAllowed(t *testing.T) {
	rl := apgw.NewRateLimiter(1, 1, time.Minute)
	for i := 0; i < 10; i++ {
		if !rl.AllowToken("") {
			t.Error("empty token should always be allowed")
		}
	}
}

func TestRateLimiter_DifferentIPs_IndependentBuckets(t *testing.T) {
	rl := apgw.NewRateLimiter(1, 100, time.Minute)
	for i := 0; i < 5; i++ {
		addr := fmt.Sprintf("192.168.0.%d:80", i)
		if !rl.AllowIP(addr) {
			t.Errorf("first request from %s should be allowed", addr)
		}
	}
}
