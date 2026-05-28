package gateway

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

func TestRemoteAddrResolver(t *testing.T) {
	t.Parallel()

	resolver := RemoteAddrResolver{}

	tests := []struct {
		name     string
		remote   string
		expected string
	}{
		{name: "ipv4_with_port", remote: "192.168.1.1:8080", expected: "192.168.1.1"},
		{name: "ipv6_with_port", remote: "[::1]:8080", expected: "::1"},
		{name: "ipv4_no_port", remote: "10.0.0.1", expected: "10.0.0.1"},
		{name: "hostname_with_port", remote: "localhost:3000", expected: "localhost"},
		{name: "empty", remote: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{RemoteAddr: tt.remote}
			got := resolver.ClientIP(req)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestForwardedForResolver_NoHeader(t *testing.T) {
	t.Parallel()

	resolver := NewForwardedForResolver()
	req := &http.Request{RemoteAddr: "10.0.0.1:5678"}
	assert.Equal(t, "10.0.0.1", resolver.ClientIP(req))
}

func TestForwardedForResolver_NoTrustedProxies(t *testing.T) {
	t.Parallel()

	resolver := NewForwardedForResolver()
	req := &http.Request{
		RemoteAddr: "10.0.0.1:5678",
		Header:     make(http.Header),
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.2, 10.0.0.1")
	// Without trusted proxies, return leftmost.
	assert.Equal(t, "203.0.113.1", resolver.ClientIP(req))
}

func TestForwardedForResolver_WithTrustedProxies(t *testing.T) {
	t.Parallel()

	resolver := NewForwardedForResolver("10.0.0.0/8", "192.168.0.0/16")

	tests := []struct {
		name   string
		xff    string
		remote string
		want   string
	}{
		{
			name:   "single_trusted_proxy",
			xff:    "203.0.113.1, 10.0.0.1",
			remote: "10.0.0.1:1234",
			want:   "203.0.113.1",
		},
		{
			name:   "multiple_trusted_proxies",
			xff:    "203.0.113.1, 192.168.1.1, 10.0.0.1",
			remote: "10.0.0.1:1234",
			want:   "203.0.113.1",
		},
		{
			name:   "all_trusted",
			xff:    "10.0.0.2, 10.0.0.1",
			remote: "10.0.0.1:1234",
			want:   "10.0.0.2", // leftmost fallback
		},
		{
			name:   "single_address",
			xff:    "203.0.113.1",
			remote: "10.0.0.1:1234",
			want:   "203.0.113.1",
		},
		{
			name:   "spaces_in_header",
			xff:    " 203.0.113.1 , 10.0.0.1 ",
			remote: "10.0.0.1:1234",
			want:   "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: tt.remote,
				Header:     make(http.Header),
			}
			req.Header.Set("X-Forwarded-For", tt.xff)
			got := resolver.ClientIP(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestForwardedForResolver_InvalidCIDR(t *testing.T) {
	t.Parallel()

	// Invalid CIDR should be silently ignored.
	resolver := NewForwardedForResolver("not-a-cidr", "10.0.0.0/8")
	require.Len(t, resolver.TrustedProxies, 1, "invalid CIDR should be ignored")
}

func TestForwardedForResolver_FirstNonTrusted(t *testing.T) {
	t.Parallel()

	// Only 10.0.0.0/8 is trusted. 192.168.1.1 is NOT trusted.
	resolver := NewForwardedForResolver("10.0.0.0/8")

	req := &http.Request{
		RemoteAddr: "10.0.0.5:9999",
		Header:     make(http.Header),
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 192.168.1.1, 10.0.0.5")
	// Walking from right: 10.0.0.5 (trusted) -> 192.168.1.1 (NOT trusted) -> return 192.168.1.1
	assert.Equal(t, "192.168.1.1", resolver.ClientIP(req))
}

func TestRealIPResolver_NoHeader(t *testing.T) {
	t.Parallel()

	resolver := NewRealIPResolver()
	req := &http.Request{RemoteAddr: "10.0.0.1:8080"}
	assert.Equal(t, "10.0.0.1", resolver.ClientIP(req))
}

func TestRealIPResolver_WithHeaderNoProxy(t *testing.T) {
	t.Parallel()

	resolver := NewRealIPResolver() // no trusted proxies = always trust header
	req := &http.Request{
		RemoteAddr: "10.0.0.1:8080",
		Header:     make(http.Header),
	}
	req.Header.Set("X-Real-IP", "203.0.113.50")
	assert.Equal(t, "203.0.113.50", resolver.ClientIP(req))
}

func TestRealIPResolver_WithTrustedProxy(t *testing.T) {
	t.Parallel()

	resolver := NewRealIPResolver("10.0.0.0/8")
	req := &http.Request{
		RemoteAddr: "10.0.0.1:8080",
		Header:     make(http.Header),
	}
	req.Header.Set("X-Real-IP", "203.0.113.50")
	// 10.0.0.1 is trusted, so honour X-Real-IP
	assert.Equal(t, "203.0.113.50", resolver.ClientIP(req))
}

func TestRealIPResolver_WithUntrustedProxy(t *testing.T) {
	t.Parallel()

	resolver := NewRealIPResolver("10.0.0.0/8")
	req := &http.Request{
		RemoteAddr: "192.168.1.1:8080",
		Header:     make(http.Header),
	}
	req.Header.Set("X-Real-IP", "203.0.113.50")
	// 192.168.1.1 is not trusted, so fall back to RemoteAddr
	assert.Equal(t, "192.168.1.1", resolver.ClientIP(req))
}

func TestRealIPResolver_EmptyRemoteAddr(t *testing.T) {
	t.Parallel()

	resolver := NewRealIPResolver("10.0.0.0/8")
	req := &http.Request{
		RemoteAddr: "",
		Header:     make(http.Header),
	}
	req.Header.Set("X-Real-IP", "203.0.113.50")
	// Empty remote addr with trusted proxies configured — still fallback
	assert.Equal(t, "", resolver.ClientIP(req))
}

func TestClientIPResolver_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// Ensure both types satisfy the interface.
	var r1 dgw.ClientIPResolver = RemoteAddrResolver{}
	var r2 dgw.ClientIPResolver = NewForwardedForResolver()
	var r3 dgw.ClientIPResolver = NewRealIPResolver()

	req := &http.Request{RemoteAddr: "1.2.3.4:5678"}
	assert.Equal(t, "1.2.3.4", r1.ClientIP(req))
	assert.Equal(t, "1.2.3.4", r2.ClientIP(req))
	assert.Equal(t, "1.2.3.4", r3.ClientIP(req))
}
