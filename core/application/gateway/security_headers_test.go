package gateway_test

import (
	"testing"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
)

func TestSecurityHeaders_ContainsRequiredKeys(t *testing.T) {
	h := apgw.SecurityHeaders()
	required := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Strict-Transport-Security",
		"Referrer-Policy",
		"Content-Security-Policy",
	}
	for _, k := range required {
		if _, ok := h[k]; !ok {
			t.Errorf("missing security header: %s", k)
		}
	}
}

func TestSecurityHeaders_XFrameOptions_IsDeny(t *testing.T) {
	h := apgw.SecurityHeaders()
	if h["X-Frame-Options"] != "DENY" {
		t.Errorf("expected X-Frame-Options=DENY, got %q", h["X-Frame-Options"])
	}
}

func TestSecurityHeaders_HSTS_SetCorrectly(t *testing.T) {
	h := apgw.SecurityHeaders()
	if h["Strict-Transport-Security"] == "" {
		t.Error("Strict-Transport-Security must not be empty")
	}
}
