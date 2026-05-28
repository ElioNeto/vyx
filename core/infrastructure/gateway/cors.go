package gateway

import (
	"fmt"
	"net/http"
	"strings"
)

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposeHeaders    []string
	MaxAge           int
	AllowCredentials bool
}

// DefaultCORSConfig returns a safe default CORS config (same-origin only).
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{}, // empty = same-origin only
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-Id", "X-Correlation-Id"},
		ExposeHeaders:    []string{"X-Request-Id", "X-Correlation-Id", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		MaxAge:           86400,
		AllowCredentials: true,
	}
}

// CORSMiddleware wraps an http.Handler with CORS headers.
func CORSMiddleware(cfg CORSConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// If no Origin header, skip CORS (not a browser request)
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Validate origin against allowed list
		if !isOriginAllowed(origin, cfg.AllowedOrigins) {
			// Origin not allowed — still process the request but don't set CORS headers
			// Browser will block the response due to missing ACAO header
			next.ServeHTTP(w, r)
			return
		}

		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")

		if cfg.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if len(cfg.AllowedMethods) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
		}

		if len(cfg.AllowedHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
		}

		if len(cfg.ExposeHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
		}

		if cfg.MaxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed checks if origin matches any entry in allowedOrigins.
// Empty allowedOrigins means same-origin only (all origins are rejected).
// "*" allows all origins.
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
	}
	return false
}
