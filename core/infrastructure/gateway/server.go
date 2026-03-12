package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

const defaultMaxBodyBytes = 1 << 20 // 1 MiB default; overridden via WithMaxBodyBytes

// Server is the HTTP gateway that exposes the vyx core to the outside world.
// It supports HTTP/1.1 and HTTP/2 (via net/http's built-in H2 support).
type Server struct {
	httpServer  *http.Server
	dispatcher  *apgw.Dispatcher
	rateLimiter *apgw.RateLimiter
	maxBodyBytes int64
	log         *zap.Logger
}

// Config holds the HTTP server configuration.
type Config struct {
	Addr         string        // e.g. ":8080"
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	MaxBodyBytes int64
}

// DefaultConfig returns production-safe HTTP server defaults.
func DefaultConfig() Config {
	return Config{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		MaxBodyBytes: defaultMaxBodyBytes,
	}
}

// New creates a Server wired with a Dispatcher and RateLimiter.
func New(
	cfg Config,
	dispatcher *apgw.Dispatcher,
	rateLimiter *apgw.RateLimiter,
	log *zap.Logger,
) *Server {
	s := &Server{
		dispatcher:   dispatcher,
		rateLimiter:  rateLimiter,
		maxBodyBytes: cfg.MaxBodyBytes,
		log:          log,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)

	s.httpServer = &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// ListenAndServe starts the HTTP server. It blocks until the server stops.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully drains active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// handle is the single entry-point handler for all requests.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	// Rate limit by IP.
	if !s.rateLimiter.AllowIP(r.RemoteAddr) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	// Rate limit by token (best-effort; token validation happens downstream).
	token := r.Header.Get("Authorization")
	if !s.rateLimiter.AllowToken(token) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	// Enforce payload size limit.
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Flatten headers.
	headers := make(map[string]string, len(r.Header))
	for k, vs := range r.Header {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}

	req := &dgw.GatewayRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: headers,
		Body:    body,
	}

	resp, err := s.dispatcher.Dispatch(r.Context(), req)
	if err != nil {
		s.writeError(w, err)
		return
	}

	// Apply security headers.
	for k, v := range apgw.SecurityHeaders() {
		w.Header().Set(k, v)
	}
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	for k, v := range apgw.SecurityHeaders() {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")

	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, dgw.ErrRouteNotFound):
		code = http.StatusNotFound
	case errors.Is(err, dgw.ErrUnauthorized):
		code = http.StatusUnauthorized
	case errors.Is(err, dgw.ErrForbidden):
		code = http.StatusForbidden
	case errors.Is(err, dgw.ErrSchemaValidation):
		code = http.StatusBadRequest
	case errors.Is(err, dgw.ErrPayloadTooLarge):
		code = http.StatusRequestEntityTooLarge
	case errors.Is(err, dgw.ErrUpstreamTimeout):
		code = http.StatusGatewayTimeout
	}

	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})

	s.log.Warn("gateway error",
		zap.Int("status", code),
		zap.Error(err),
	)
}

// Addr returns the address the server is configured to listen on.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// FormatUptime is a helper kept for future metrics middleware.
func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%s", d.Round(time.Second))
}
