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
	Body    []byte
	Claims  *Claims // nil when the route requires no auth
}

// GatewayResponse holds the worker's reply to be sent back to the HTTP client.
type GatewayResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}
