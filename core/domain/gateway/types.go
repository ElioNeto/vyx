package gateway

// Claims holds the verified payload extracted from a JWT.
type Claims struct {
	UserID string
	Roles  []string
}

// GatewayRequest is the normalised, transport-agnostic request passed
// through the gateway pipeline.
type GatewayRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Query   map[string]string // populated from URL query string (#37)
	Params  map[string]string // populated from path parameters (#36)
	Body    []byte
	Claims  *Claims // nil when the route requires no auth
}

// GatewayResponse holds the worker's reply to be sent back to the HTTP client.
type GatewayResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// WorkerResponse is the structured envelope that workers must return.
// The Dispatcher deserialises the IPC payload into this struct (#39).
type WorkerResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}
