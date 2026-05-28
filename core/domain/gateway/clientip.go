package gateway

import "net/http"

// ClientIPResolver extracts the real client IP from an HTTP request.
// Implementations may use RemoteAddr, X-Forwarded-For, X-Real-IP, or
// Forwarded headers, with optional trusted proxy CIDR filtering.
type ClientIPResolver interface {
	// ClientIP returns the resolved client IP address string.
	ClientIP(r *http.Request) string
}
