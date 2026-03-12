package gateway

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/net/websocket"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

const defaultMaxBodyBytes = 1 << 20 // 1 MiB

// Server is the HTTP gateway supporting HTTP/1.1, HTTP/2 (TLS) and h2c (cleartext).
type Server struct {
	httpServer   *http.Server
	dispatcher   *apgw.Dispatcher
	rateLimiter  *apgw.RateLimiter
	wsProxy      *wsProxy
	maxBodyBytes int64
	log          *zap.Logger
}

// Config holds the HTTP server configuration.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	MaxBodyBytes int64
	// TLS fields — both must be set to enable TLS + H2.
	TLSCertFile string
	TLSKeyFile  string
	// H2CEnabled enables HTTP/2 cleartext (development mode).
	H2CEnabled bool
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

// DevConfig returns a development config with h2c enabled and verbose defaults.
func DevConfig() Config {
	cfg := DefaultConfig()
	cfg.H2CEnabled = true
	return cfg
}

// New creates a Server wired with a Dispatcher, RateLimiter, and WebSocket proxy.
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

	// WebSocket proxy wired from dispatcher dependencies (#19).
	s.wsProxy = newWSProxy(
		dispatcher.Routes(),
		dispatcher.Transport(),
		dispatcher.JWT(),
		log,
		dispatcher.Timeout(),
	)

	mux := http.NewServeMux()
	// WebSocket routes are served under /ws/* (#19).
	mux.Handle("/ws/", s.wsProxy)
	mux.HandleFunc("/", s.handle)

	var handler http.Handler = mux

	// Wrap with h2c handler for HTTP/2 cleartext (dev mode).
	if cfg.H2CEnabled && cfg.TLSCertFile == "" {
		h2s := &http2.Server{}
		handler = h2c.NewHandler(mux, h2s)
	}

	s.httpServer = &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: 0, // 0 = no write deadline for WebSocket long-lived connections
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Configure HTTP/2 over TLS.
	if cfg.TLSCertFile != "" {
		if err := http2.ConfigureServer(s.httpServer, &http2.Server{}); err != nil {
			log.Warn("http2.ConfigureServer failed", zap.Error(err))
		}
		s.httpServer.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2", "http/1.1"},
		}
	}

	return s
}

// ListenAndServe starts the HTTP/1.1 (or h2c) server. Blocks until stopped.
func (s *Server) ListenAndServe() error {
	s.log.Info("HTTP server listening", zap.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// ListenAndServeTLS starts an HTTPS server with H2 over ALPN. Blocks until stopped.
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	s.log.Info("HTTPS/H2 server listening", zap.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully drains active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Addr returns the address the server listens on.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// handle is the single entry-point handler for regular HTTP requests.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	// Detect WebSocket upgrades and hand off to the proxy (#19).
	if isWebSocketUpgrade(r) {
		s.wsProxy.ServeHTTP(w, r)
		return
	}

	if !s.rateLimiter.AllowIP(r.RemoteAddr) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	token := r.Header.Get("Authorization")
	if !s.rateLimiter.AllowToken(token) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	headers := make(map[string]string, len(r.Header))
	for k, vs := range r.Header {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}

	queryParams := make(map[string]string, len(r.URL.Query()))
	for k, vs := range r.URL.Query() {
		if len(vs) > 0 {
			queryParams[k] = vs[0]
		}
	}

	req := &dgw.GatewayRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: headers,
		Query:   queryParams,
		Body:    body,
	}

	resp, err := s.dispatcher.Dispatch(r.Context(), req)
	if err != nil {
		s.writeError(w, err)
		return
	}

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
		// Render the structured ValidationError body if available.
		var ve *dgw.ValidationError
		if errors.As(err, &ve) {
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(ve)
			s.log.Warn("gateway validation error", zap.Int("status", code), zap.Error(err))
			return
		}
	case errors.Is(err, dgw.ErrPayloadTooLarge):
		code = http.StatusRequestEntityTooLarge
	case errors.Is(err, dgw.ErrUpstreamTimeout):
		code = http.StatusGatewayTimeout
	}

	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	s.log.Warn("gateway error", zap.Int("status", code), zap.Error(err))
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%s", d.Round(time.Second))
}

// Ensure websocket import is used (compiler check).
var _ = websocket.Handler
var _ ipc.MessageType = ipc.TypeWSOpen
