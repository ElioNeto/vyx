package gateway

// Claims holds the verified payload extracted from a JWT.
type Claims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

// GatewayRequest is the normalised, transport-agnostic request passed
// through the gateway pipeline.
type GatewayRequest struct {
	Method   string
	Path     string
	Headers  map[string]string
	Query    map[string]string // populated from URL query string (#37)
	Params   map[string]string // populated from path parameters (#36)
	Body     []byte
	Claims   *Claims // nil when the route requires no auth
	ClientIP string // real client IP resolved by ClientIPResolver (#57)
}

// GatewayResponse holds the worker's reply to be sent back to the HTTP client.
type GatewayResponse struct {
	StatusCode    int
	Headers       map[string]string
	Body          []byte
	CorrelationID string // echoed as X-Request-Id response header (#52)
}

// WorkerResponse is the structured envelope that workers must return.
// The Dispatcher deserialises the IPC payload into this struct (#39).
// Body is any because workers may send a JSON string (Node.js HTML pages),
// a JSON object (Go/Python API responses), or a JSON array.
// We convert to []byte in processWorkerResponse based on the underlying type.
type WorkerResponse struct {
	StatusCode    int               `json:"status_code"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          any               `json:"body,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
}
