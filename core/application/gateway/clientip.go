package gateway

import (
	"net"
	"net/http"
	"strings"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// RemoteAddrResolver returns the TCP peer address (r.RemoteAddr) as-is.
// It strips the port portion before returning.
type RemoteAddrResolver struct{}

// ClientIP extracts the IP from r.RemoteAddr, stripping the port.
func (RemoteAddrResolver) ClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// compile-time interface check
var _ dgw.ClientIPResolver = (*RemoteAddrResolver)(nil)

// ForwardedForResolver extracts the client IP from the X-Forwarded-For
// header, falling back to RemoteAddr.  When TrustedProxies is non-empty
// only the rightmost trusted proxy's left neighbour is returned;
// otherwise the leftmost address in the chain is used.
//
// X-Forwarded-For: <client>, <proxy1>, <proxy2>
type ForwardedForResolver struct {
	TrustedProxies []net.IPNet // CIDR blocks of trusted reverse proxies
	fallback       dgw.ClientIPResolver
}

// NewForwardedForResolver creates a resolver with the given trusted proxy CIDRs.
// If no CIDRs are provided, the leftmost address is used (unsafe in production
// when behind a reverse proxy).
func NewForwardedForResolver(trustedCIDRs ...string) *ForwardedForResolver {
	r := &ForwardedForResolver{
		fallback: RemoteAddrResolver{},
	}
	for _, cidr := range trustedCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err == nil {
			r.TrustedProxies = append(r.TrustedProxies, *n)
		}
	}
	return r
}

// ClientIP extracts the real client IP from X-Forwarded-For.
func (r *ForwardedForResolver) ClientIP(req *http.Request) string {
	xff := req.Header.Get("X-Forwarded-For")
	if xff == "" {
		return r.fallback.ClientIP(req)
	}

	addrs := strings.Split(xff, ",")
	for i := range addrs {
		addrs[i] = strings.TrimSpace(addrs[i])
	}

	if len(r.TrustedProxies) == 0 {
		// No trusted proxies configured — return leftmost address.
		return addrs[0]
	}

	// Walk from right to left, returning the first non-trusted address.
	// If all addresses are trusted, return the leftmost as fallback.
	for i := len(addrs) - 1; i >= 0; i-- {
		ip := net.ParseIP(addrs[i])
		if ip == nil {
			continue
		}
		if !r.isTrusted(ip) {
			return addrs[i]
		}
	}
	return addrs[0]
}

func (r *ForwardedForResolver) isTrusted(ip net.IP) bool {
	for _, cidr := range r.TrustedProxies {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// compile-time interface check
var _ dgw.ClientIPResolver = (*ForwardedForResolver)(nil)

// RealIPResolver extracts the client IP from the X-Real-IP header,
// falling back to RemoteAddrResolve.  This is common when behind
// NGINX or similar reverse proxies that set this header.
type RealIPResolver struct {
	TrustedProxies []net.IPNet
	fallback       dgw.ClientIPResolver
}

// NewRealIPResolver creates a resolver that prefers X-Real-IP.
// When TrustedProxies is non-empty, the X-Real-IP header is only
// honoured if the immediate peer (RemoteAddr) is in the trusted set.
func NewRealIPResolver(trustedCIDRs ...string) *RealIPResolver {
	r := &RealIPResolver{
		fallback: RemoteAddrResolver{},
	}
	for _, cidr := range trustedCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err == nil {
			r.TrustedProxies = append(r.TrustedProxies, *n)
		}
	}
	return r
}

// ClientIP returns the X-Real-IP header value if the peer is trusted
// (or no proxies are configured), otherwise falls back to RemoteAddr.
func (r *RealIPResolver) ClientIP(req *http.Request) string {
	realIP := req.Header.Get("X-Real-IP")
	if realIP == "" {
		return r.fallback.ClientIP(req)
	}

	if len(r.TrustedProxies) == 0 {
		return realIP
	}

	// Only honour X-Real-IP when the immediate peer is trusted.
	peerIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil || peerIP == "" {
		return r.fallback.ClientIP(req)
	}

	parsed := net.ParseIP(peerIP)
	if parsed == nil {
		return r.fallback.ClientIP(req)
	}

	for _, cidr := range r.TrustedProxies {
		if cidr.Contains(parsed) {
			return realIP
		}
	}

	return r.fallback.ClientIP(req)
}

// compile-time interface check
var _ dgw.ClientIPResolver = (*RealIPResolver)(nil)
