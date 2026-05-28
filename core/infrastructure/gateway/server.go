package gateway

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	dmetrics "github.com/ElioNeto/vyx/core/domain/metrics"
)

const defaultMaxBodyBytes = 1 << 20 // 1 MiB
const headerXRequestID = "X-Request-Id"

const maxHeaderValueLen = 4096

// sanitizeHeaderValue strips control characters (< 0x20) except tab (0x09),
// removing \r (0x0d) and \n (0x0a) which enable HTTP response splitting,
// and truncates values exceeding maxHeaderValueLen.
func sanitizeHeaderValue(value string) string {
	if len(value) > maxHeaderValueLen {
		value = value[:maxHeaderValueLen]
	}
	buf := make([]byte, 0, len(value))
	for i := range len(value) {
		b := value[i]
		if b == '\t' || b >= 0x20 {
			buf = append(buf, b)
		}
	}
	return string(buf)
}

// Server is the HTTP gateway supporting HTTP/1.1, HTTP/2 (TLS) and h2c (cleartext).
type Server struct {
	httpServer    *http.Server
	dispatcher    *apgw.Dispatcher
	rateLimiter   *apgw.RateLimiter
	wsProxy       *wsProxy
	maxBodyBytes  int64
	log           *zap.Logger
	ipResolver    dgw.ClientIPResolver
	metricsProv   dmetrics.Provider
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

// New creates a Server wired with a Dispatcher, RateLimiter, WebSocket proxy,
// and ClientIPResolver.  Pass nil for ipResolver to use the default
// RemoteAddrResolver.  Pass nil for metricsProv to disable metrics endpoint.
func New(
	cfg Config,
	dispatcher *apgw.Dispatcher,
	rateLimiter *apgw.RateLimiter,
	log *zap.Logger,
	ipResolver dgw.ClientIPResolver,
	metricsProv dmetrics.Provider,
) *Server {
	if ipResolver == nil {
		ipResolver = apgw.RemoteAddrResolver{}
	}
	if metricsProv == nil {
		metricsProv = dmetrics.Noop()
	}
	s := &Server{
		dispatcher:   dispatcher,
		rateLimiter:  rateLimiter,
		maxBodyBytes: cfg.MaxBodyBytes,
		log:          log,
		ipResolver:   ipResolver,
		metricsProv:  metricsProv,
	}

	// WebSocket proxy wired from dispatcher accessors.
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
	// Metrics endpoint: GET /metrics.
	if h := metricsProv.HTTPHandler(); h != nil {
		if handler, ok := h.(http.Handler); ok {
			mux.Handle("/metrics", handler)
		}
	}
	mux.HandleFunc("/", s.handle)

	var handler http.Handler = mux

	// Wrap with h2c handler for HTTP/2 cleartext (dev mode).
	if cfg.H2CEnabled && cfg.TLSCertFile == "" {
		h2s := &http2.Server{}
		handler = h2c.NewHandler(mux, h2s)
	}

	// Apply CORS middleware (outermost wrapper).
	handler = CORSMiddleware(DefaultCORSConfig(), handler)

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
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
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

// contentTypeErrorBody returns a JSON error body for 415 Unsupported Media Type.
func contentTypeErrorBody() []byte {
	b, _ := json.Marshal(map[string]string{
		"error": "unsupported media type, only application/json is accepted",
	})
	return b
}

// checkContentType validates that POST, PUT and PATCH requests have
// Content-Type: application/json.  Returns false and writes a 415 response
// when the media type is missing or not application/json.
func (s *Server) checkContentType(w http.ResponseWriter, r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		ct := r.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			writeUnchecked(w, contentTypeErrorBody())
			return false
		}
	}
	return true
}

// allowedMethods is the set of HTTP methods accepted by the gateway.
// HEAD is handled automatically by Go's http.ServeMux for GET routes.
// OPTIONS is allowed for CORS preflight (already handled by CORSMiddleware).
var allowedMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

// methodNotAllowedBody returns a JSON error body for 405 Method Not Allowed.
func methodNotAllowedBody() []byte {
	b, _ := json.Marshal(map[string]string{
		"error": "method not allowed",
	})
	return b
}

// checkMethod validates that the request HTTP method is in the allowlist.
// Returns false and writes a 405 JSON response when the method is not allowed.
func (s *Server) checkMethod(w http.ResponseWriter, r *http.Request) bool {
	if !allowedMethods[r.Method] {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		writeUnchecked(w, methodNotAllowedBody())
		return false
	}
	return true
}

// handle is the single entry-point handler for regular HTTP requests.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if isWebSocketUpgrade(r) {
		s.wsProxy.ServeHTTP(w, r)
		return
	}

	if !s.checkMethod(w, r) {
		return
	}

	if !s.checkRateLimit(w, r) {
		return
	}

	if !s.checkContentType(w, r) {
		return
	}

	body, err := s.readBody(w, r)
	if err != nil {
		return
	}

	req := s.buildGatewayRequest(r, body)
	resp, err := s.dispatcher.Dispatch(r.Context(), req)
	if err != nil {
		s.handleDispatchError(w, r, err)
		return
	}

	s.writeResponse(w, resp)
}

func (s *Server) checkRateLimit(w http.ResponseWriter, r *http.Request) bool {
	clientIP := s.ipResolver.ClientIP(r)
	if ok, retryAfter := s.rateLimiter.AllowIP(clientIP); !ok {
		secs := int(math.Ceil(retryAfter.Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		writeUnchecked(w, rateLimitBody(secs))
		return false
	}
	if ok, retryAfter := s.rateLimiter.AllowToken(r.Header.Get("Authorization")); !ok {
		secs := int(math.Ceil(retryAfter.Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		writeUnchecked(w, rateLimitBody(secs))
		return false
	}
	return true
}

func rateLimitBody(retryAfter int) []byte {
	b, _ := json.Marshal(map[string]any{
		"error":       "too many requests",
		"retry_after": retryAfter,
	})
	return b
}

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return nil, err
	}
	return body, nil
}

func (s *Server) buildGatewayRequest(r *http.Request, body []byte) *dgw.GatewayRequest {
	headers := make(map[string]string, len(r.Header))
	for k, vs := range r.Header {
		if len(vs) > 0 {
			headers[k] = sanitizeHeaderValue(vs[0])
		}
	}

	queryParams := make(map[string]string, len(r.URL.Query()))
	for k, vs := range r.URL.Query() {
		if len(vs) > 0 {
			queryParams[k] = vs[0]
		}
	}

	return &dgw.GatewayRequest{
		Method:   r.Method,
		Path:     r.URL.Path,
		Headers:  headers,
		Query:    queryParams,
		Body:     body,
		ClientIP: s.ipResolver.ClientIP(r),
	}
}

func (s *Server) handleDispatchError(w http.ResponseWriter, r *http.Request, err error) {
	if corrID := r.Header.Get(headerXRequestID); corrID != "" {
		w.Header().Set(headerXRequestID, corrID)
	}
	s.writeError(w, err)
}

func (s *Server) writeResponse(w http.ResponseWriter, resp *dgw.GatewayResponse) {
	for k, v := range apgw.SecurityHeaders() {
		w.Header().Set(k, v)
	}
	for k, v := range resp.Headers {
		w.Header().Set(k, sanitizeHeaderValue(v))
	}
	w.Header().Set("Content-Type", "application/json")
	if resp.CorrelationID != "" {
		w.Header().Set(headerXRequestID, resp.CorrelationID)
	}
	w.WriteHeader(resp.StatusCode)
	writeUnchecked(w, resp.Body)
}

// safeErrorMessages maps known sentinel errors to safe, user-facing messages.
// The raw err.Error() must never be sent to clients as it may leak internal
// implementation details (file paths, dependency errors, etc.).
var safeErrorMessages = map[error]string{
	dgw.ErrRouteNotFound:   "route not found",
	dgw.ErrUnauthorized:    "unauthorized",
	dgw.ErrForbidden:       "forbidden",
	dgw.ErrPayloadTooLarge: "payload too large",
	dgw.ErrUpstreamTimeout: "upstream timeout",
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	for k, v := range apgw.SecurityHeaders() {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")

	code := http.StatusInternalServerError
	message := "internal server error"

	switch {
	case errors.Is(err, dgw.ErrRouteNotFound):
		code = http.StatusNotFound
		message = safeErrorMessages[dgw.ErrRouteNotFound]
	case errors.Is(err, dgw.ErrUnauthorized):
		code = http.StatusUnauthorized
		message = safeErrorMessages[dgw.ErrUnauthorized]
	case errors.Is(err, dgw.ErrForbidden):
		code = http.StatusForbidden
		message = safeErrorMessages[dgw.ErrForbidden]
	case errors.Is(err, dgw.ErrSchemaValidation):
		code = http.StatusBadRequest
		var ve *dgw.ValidationError
		if errors.As(err, &ve) {
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(ve)
			s.log.Warn("gateway validation error", zap.Int("status", code), zap.Error(err))
			return
		}
		message = "validation failed"
	case errors.Is(err, dgw.ErrPayloadTooLarge):
		code = http.StatusRequestEntityTooLarge
		message = safeErrorMessages[dgw.ErrPayloadTooLarge]
	case errors.Is(err, dgw.ErrUpstreamTimeout):
		code = http.StatusGatewayTimeout
		message = safeErrorMessages[dgw.ErrUpstreamTimeout]
	}

	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
	s.log.Warn("gateway error", zap.Int("status", code), zap.Error(err))
}

// writeUnchecked writes body to the response writer, ignoring any write error.
// The connection is already broken if Write fails; there is nothing to do.
// This helper exists to satisfy linters that flag unchecked error returns.
func writeUnchecked(w http.ResponseWriter, body []byte) {
	_, _ = w.Write(body)
}
