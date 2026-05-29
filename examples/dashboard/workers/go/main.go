// Go worker for the dashboard vyx example — User Management API.
//
// This worker connects to the vyx core via Unix Domain Socket (UDS) on Unix
// or via Named Pipe on Windows, performs the handshake, and handles requests.
//
// Annotated routes (parsed at build time by `vyx build`):
//
// @Route(POST /api/auth/login)
// @Validate(JsonSchema: "login")
// @Auth(roles: ["guest"])
//
// @Route(GET /api/users)
// @Auth(roles: ["admin"])
//
// @Route(GET /api/users/:id)
// @Auth(roles: ["admin", "user"])
//
// @Route(POST /api/users)
// @Validate(JsonSchema: "user")
// @Auth(roles: ["admin"])
//
// @Route(PUT /api/users/:id)
// @Validate(JsonSchema: "user")
// @Auth(roles: ["admin", "user"])
//
// @Route(GET /api/users/:id/settings)
// @Auth(roles: ["admin", "user"])
//
// @Route(PUT /api/users/:id/settings)
// @Validate(JsonSchema: "settings")
// @Auth(roles: ["admin", "user"])
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ─── IPC protocol types (mirrors core/domain/ipc/message.go) ─────────────────

const (
	typeRequest   = 0x01
	typeResponse  = 0x02
	typeHeartbeat = 0x03
	typeError     = 0x04
	typeHandshake = 0x05 // must match ipc.TypeHandshake in core/domain/ipc/message.go
)

type frame struct {
	Length  uint32
	MsgType uint8
	Payload []byte
}

// handshakePayload mirrors ipc.HandshakePayload in core/domain/ipc/message.go.
// Note: no "type" field — the frame type byte already encodes the message kind.
type handshakePayload struct {
	WorkerID     string       `json:"worker_id"`
	Capabilities []capability `json:"capabilities"`
}

type capability struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

type request struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Params  map[string]string `json:"params"`
	Body    json.RawMessage   `json:"body"`
	Claims  map[string]any    `json:"claims"`
}

type response struct {
	StatusCode    int               `json:"status_code"`
	Headers       map[string]string `json:"headers"`
	Body          any               `json:"body"`
	CorrelationID string            `json:"correlation_id,omitempty"`
}

// ─── Domain types ─────────────────────────────────────────────────────────────

type user struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"-"`
}

type settings struct {
	Theme               string `json:"theme"`
	NotificationsEnabled bool  `json:"notifications_enabled"`
	Language            string `json:"language"`
	Timezone            string `json:"timezone"`
}

type userPublic struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string     `json:"token"`
	User  userPublic `json:"user"`
}

// ─── In-memory stores ─────────────────────────────────────────────────────────

var (
	mu           sync.Mutex
	users        = map[string]user{}
	userSettings = map[string]settings{}
	nextID       = 3
)

func init() {
	// Pre-seeded users
	users["1"] = user{
		ID:       "1",
		Name:     "admin",
		Email:    "admin@dashboard.local",
		Role:     "admin",
		Password: "admin123",
	}
	users["2"] = user{
		ID:       "2",
		Name:     "user",
		Email:    "user@dashboard.local",
		Role:     "user",
		Password: "password",
	}

	// Default settings for seeded users
	userSettings["1"] = settings{
		Theme:                "dark",
		NotificationsEnabled: true,
		Language:             "en",
		Timezone:             "UTC",
	}
	userSettings["2"] = settings{
		Theme:                "light",
		NotificationsEnabled: false,
		Language:             "en",
		Timezone:             "America/New_York",
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// writeFrame encodes a frame using little-endian length prefix (4 bytes) +
// msgType (1 byte) + payload, matching framing.go in the vyx core.
func writeFrame(conn net.Conn, msgType uint8, payload []byte) error {
	header := make([]byte, 5)
	binary.LittleEndian.PutUint32(header[0:4], uint32(len(payload)))
	header[4] = msgType
	_, err := conn.Write(append(header, payload...))
	return err
}

// readFrame reads a little-endian framed message from conn.
func readFrame(conn net.Conn) (frame, error) {
	header := make([]byte, 5)
	if _, err := readFull(conn, header); err != nil {
		return frame{}, err
	}
	length := binary.LittleEndian.Uint32(header[0:4])
	msgType := header[4]
	payload := make([]byte, length)
	if length > 0 {
		if _, err := readFull(conn, payload); err != nil {
			return frame{}, err
		}
	}
	return frame{Length: length, MsgType: msgType, Payload: payload}, nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// dialSocket connects to the core via UDS (Unix) or Named Pipe (Windows).
func dialSocket(socketPath string) (net.Conn, error) {
	if runtime.GOOS == "windows" {
		// On Windows the core passes \\.\pipe\vyx-<id> as --vyx-socket.
		// net.Dial does not support the namedpipe scheme, so we use the
		// platform-specific helper defined in dial_windows.go.
		return dialNamedPipe(socketPath)
	}
	return net.Dial("unix", socketPath)
}

// toPublic strips the password from a user.
func toPublic(u user) userPublic {
	return userPublic{
		ID:    u.ID,
		Name:  u.Name,
		Email: u.Email,
		Role:  u.Role,
	}
}

// ─── Route handlers ───────────────────────────────────────────────────────────

// @Route(POST /api/auth/login)
// @Validate(JsonSchema: "login")
// @Auth(roles: ["guest"])
func handleLogin(req request) response {
	var body loginRequest

	// Try JSON first, then form-encoded (for browsers without JS).
	ct := req.Headers["Content-Type"]
	isForm := strings.Contains(ct, "application/x-www-form-urlencoded")
	if isForm {
		// The body is sent as a JSON-escaped string. Unmarshal it first.
		var rawBody string
		if err := json.Unmarshal(req.Body, &rawBody); err != nil {
			rawBody = string(req.Body)
		}
		vals, err := url.ParseQuery(rawBody)
		if err != nil {
			return response{
				StatusCode: 400,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"error": "invalid form data"},
			}
		}
		body.Username = vals.Get("username")
		body.Password = vals.Get("password")
	} else {
		if err := json.Unmarshal(req.Body, &body); err != nil {
			return response{
				StatusCode: 400,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"error": "invalid request body"},
			}
		}
	}

	mu.Lock()
	var foundUser *user
	for _, u := range users {
		if u.Name == body.Username && u.Password == body.Password {
			u := u
			foundUser = &u
			break
		}
	}
	mu.Unlock()

	if foundUser == nil {
		return response{
			StatusCode: 401,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "invalid credentials"},
		}
	}

	token, err := generateJWT(foundUser.ID, foundUser.Role, foundUser.Name)
	if err != nil {
		return response{
			StatusCode: 500,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "failed to generate token"},
		}
	}

	if isForm {
		// Browser form submission — set cookie and redirect to dashboard.
		return response{
			StatusCode: 302,
			Headers: map[string]string{
				"Location":              "/dashboard",
				"Set-Cookie":            "vyx_token=" + token + "; Path=/; HttpOnly; SameSite=Lax",
				"Content-Type":          "text/html; charset=utf-8",
			},
			Body: "<!DOCTYPE html><html><body>Redirecting to dashboard...</body></html>",
		}
	}

	// API JSON response for fetch/XHR clients.
	resp := loginResponse{
		Token: token,
		User:  toPublic(*foundUser),
	}
	return response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Set-Cookie":   "vyx_token=" + token + "; Path=/; HttpOnly; SameSite=Lax",
		},
		Body: resp,
	}
}

// generateJWT creates a valid HS256 JWT signed with JWT_SECRET env var.
// The generated token is compatible with core/infrastructure/gateway/jwt.go.
func generateJWT(sub, role, name string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET not set")
	}

	header := `{"alg":"HS256","typ":"JWT"}`
	now := time.Now()
	payload := fmt.Sprintf(
		`{"sub":"%s","name":"%s","roles":["%s"],"iat":%d,"exp":%d}`,
		sub, name, role, now.Unix(), now.Add(24*time.Hour).Unix(),
	)

	b64 := func(data string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(data))
	}

	signingInput := b64(header) + "." + b64(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

// @Route(GET /api/users)
// @Auth(roles: ["admin"])
func handleListUsers(req request) response {
	mu.Lock()
	userList := make([]userPublic, 0, len(users))
	for _, u := range users {
		userList = append(userList, toPublic(u))
	}
	mu.Unlock()

	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       map[string]any{"users": userList, "total": len(userList)},
	}
}

// @Route(GET /api/users/:id)
// @Auth(roles: ["admin", "user"])
func handleGetUser(req request) response {
	requestedID := req.Params["id"]
	if requestedID == "" {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "missing user id"},
		}
	}

	// Users can only access their own profile; admin can access any.
	if sub, ok := req.Claims["sub"]; ok {
		callerID := fmt.Sprintf("%v", sub)
		callerRole, _ := req.Claims["role"].(string)
		if callerID != requestedID && callerRole != "admin" {
			return response{
				StatusCode: 403,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"error": "forbidden: you can only access your own profile"},
			}
		}
	}

	mu.Lock()
	u, ok := users[requestedID]
	mu.Unlock()

	if !ok {
		return response{
			StatusCode: 404,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "user not found"},
		}
	}

	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       toPublic(u),
	}
}

// @Route(POST /api/users)
// @Validate(JsonSchema: "user")
// @Auth(roles: ["admin"])
func handleCreateUser(req request) response {
	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "invalid request body"},
		}
	}

	if body.Role == "" {
		body.Role = "user"
	}

	mu.Lock()
	id := fmt.Sprintf("%d", nextID)
	nextID++
	u := user{
		ID:       id,
		Name:     body.Name,
		Email:    body.Email,
		Role:     body.Role,
		Password: "password", // default password
	}
	users[id] = u
	userSettings[id] = settings{
		Theme:                "system",
		NotificationsEnabled: true,
		Language:             "en",
		Timezone:             "UTC",
	}
	mu.Unlock()

	return response{
		StatusCode: 201,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       toPublic(u),
	}
}

// @Route(PUT /api/users/:id)
// @Validate(JsonSchema: "user")
// @Auth(roles: ["admin", "user"])
func handleUpdateUser(req request) response {
	requestedID := req.Params["id"]
	if requestedID == "" {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "missing user id"},
		}
	}

	// Users can only update their own profile; admin can update any.
	if sub, ok := req.Claims["sub"]; ok {
		callerID := fmt.Sprintf("%v", sub)
		callerRole, _ := req.Claims["role"].(string)
		if callerID != requestedID && callerRole != "admin" {
			return response{
				StatusCode: 403,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"error": "forbidden: you can only update your own profile"},
			}
		}
	}

	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "invalid request body"},
		}
	}

	mu.Lock()
	u, ok := users[requestedID]
	if !ok {
		mu.Unlock()
		return response{
			StatusCode: 404,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "user not found"},
		}
	}
	if body.Name != "" {
		u.Name = body.Name
	}
	if body.Email != "" {
		u.Email = body.Email
	}
	if body.Role != "" {
		u.Role = body.Role
	}
	users[requestedID] = u
	mu.Unlock()

	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       toPublic(u),
	}
}

// @Route(GET /api/users/:id/settings)
// @Auth(roles: ["admin", "user"])
func handleGetSettings(req request) response {
	requestedID := req.Params["id"]
	if requestedID == "" {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "missing user id"},
		}
	}

	// Users can only access their own settings; admin can access any.
	if sub, ok := req.Claims["sub"]; ok {
		callerID := fmt.Sprintf("%v", sub)
		callerRole, _ := req.Claims["role"].(string)
		if callerID != requestedID && callerRole != "admin" {
			return response{
				StatusCode: 403,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"error": "forbidden: you can only access your own settings"},
			}
		}
	}

	mu.Lock()
	s, ok := userSettings[requestedID]
	mu.Unlock()

	if !ok {
		return response{
			StatusCode: 404,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "settings not found"},
		}
	}

	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       s,
	}
}

// @Route(PUT /api/users/:id/settings)
// @Validate(JsonSchema: "settings")
// @Auth(roles: ["admin", "user"])
func handleUpdateSettings(req request) response {
	requestedID := req.Params["id"]
	if requestedID == "" {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "missing user id"},
		}
	}

	// Users can only update their own settings; admin can update any.
	if sub, ok := req.Claims["sub"]; ok {
		callerID := fmt.Sprintf("%v", sub)
		callerRole, _ := req.Claims["role"].(string)
		if callerID != requestedID && callerRole != "admin" {
			return response{
				StatusCode: 403,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"error": "forbidden: you can only update your own settings"},
			}
		}
	}

	var body struct {
		Theme               *string `json:"theme"`
		NotificationsEnabled *bool  `json:"notifications_enabled"`
		Language            *string `json:"language"`
		Timezone            *string `json:"timezone"`
	}
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "invalid request body"},
		}
	}

	mu.Lock()
	s, ok := userSettings[requestedID]
	if !ok {
		mu.Unlock()
		return response{
			StatusCode: 404,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "settings not found"},
		}
	}
	if body.Theme != nil {
		s.Theme = *body.Theme
	}
	if body.NotificationsEnabled != nil {
		s.NotificationsEnabled = *body.NotificationsEnabled
	}
	if body.Language != nil {
		s.Language = *body.Language
	}
	if body.Timezone != nil {
		s.Timezone = *body.Timezone
	}
	userSettings[requestedID] = s
	mu.Unlock()

	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       s,
	}
}

// ─── Dispatcher ───────────────────────────────────────────────────────────────

func dispatch(req request) response {
	path := strings.TrimSuffix(req.Path, "/")
	if path == "" {
		path = "/"
	}

	switch {
	case req.Method == "POST" && path == "/api/auth/login":
		return handleLogin(req)

	case req.Method == "GET" && path == "/api/users":
		return handleListUsers(req)

	case req.Method == "GET" && strings.HasPrefix(path, "/api/users/") && strings.Count(path, "/") == 3 && !strings.HasSuffix(path, "/settings"):
		// GET /api/users/:id
		parts := strings.Split(path, "/")
		if len(parts) == 4 {
			req.Params = map[string]string{"id": parts[3]}
			return handleGetUser(req)
		}

	case req.Method == "POST" && path == "/api/users":
		return handleCreateUser(req)

	case req.Method == "PUT" && strings.HasPrefix(path, "/api/users/") && strings.Count(path, "/") == 3 && !strings.HasSuffix(path, "/settings"):
		// PUT /api/users/:id
		parts := strings.Split(path, "/")
		if len(parts) == 4 {
			req.Params = map[string]string{"id": parts[3]}
			return handleUpdateUser(req)
		}

	case req.Method == "GET" && strings.HasSuffix(path, "/settings") && strings.HasPrefix(path, "/api/users/"):
		// GET /api/users/:id/settings
		parts := strings.Split(strings.TrimSuffix(path, "/settings"), "/")
		if len(parts) == 4 {
			req.Params = map[string]string{"id": parts[3]}
			return handleGetSettings(req)
		}

	case req.Method == "PUT" && strings.HasSuffix(path, "/settings") && strings.HasPrefix(path, "/api/users/"):
		// PUT /api/users/:id/settings
		parts := strings.Split(strings.TrimSuffix(path, "/settings"), "/")
		if len(parts) == 4 {
			req.Params = map[string]string{"id": parts[3]}
			return handleUpdateSettings(req)
		}
	}

	return response{
		StatusCode: 404,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       map[string]string{"error": "route not found"},
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	defaultSocket := "/tmp/vyx/go:api.sock"
	if runtime.GOOS == "windows" {
		defaultSocket = `\\.\pipe\vyx-go:api`
	}
	socketPath := flag.String("vyx-socket", defaultSocket, "IPC address provided by vyx core")
	flag.Parse()

	conn, err := dialSocket(*socketPath)
	if err != nil {
		log.Fatalf("[go:api] failed to connect to core: %v", err)
	}
	defer func() { _ = conn.Close() }()
	log.Printf("[go:api] connected to core via %s", *socketPath)

	// Handshake — payload mirrors ipc.HandshakePayload (no "type" field).
	handshake := handshakePayload{
		WorkerID: "go:api",
		Capabilities: []capability{
			{Path: "/api/auth/login", Method: "POST"},
			{Path: "/api/users", Method: "GET"},
			{Path: "/api/users/:id", Method: "GET"},
			{Path: "/api/users", Method: "POST"},
			{Path: "/api/users/:id", Method: "PUT"},
			{Path: "/api/users/:id/settings", Method: "GET"},
			{Path: "/api/users/:id/settings", Method: "PUT"},
		},
	}
	hsPayload, _ := json.Marshal(handshake)
	if err := writeFrame(conn, typeHandshake, hsPayload); err != nil {
		log.Fatalf("[go:api] handshake failed: %v", err)
	}
	log.Printf("[go:api] handshake sent")

	// Send an immediate heartbeat so the core marks this worker healthy
	// before the first 5-second monitor tick fires.
	if err := writeFrame(conn, typeHeartbeat, nil); err != nil {
		log.Fatalf("[go:api] initial heartbeat failed: %v", err)
	}
	log.Printf("[go:api] initial heartbeat sent")

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[go:api] shutting down")
		_ = conn.Close()
		os.Exit(0)
	}()

	// Main loop
	for {
		f, err := readFrame(conn)
		if err != nil {
			log.Printf("[go:api] connection closed: %v", err)
			return
		}

		switch f.MsgType {
		case typeHeartbeat:
			_ = writeFrame(conn, typeHeartbeat, nil)

		case typeRequest:
			var req request
			if err := json.Unmarshal(f.Payload, &req); err != nil {
				log.Printf("[go:api] failed to parse request: %v", err)
				continue
			}
			log.Printf("[go:api] %s %s", req.Method, req.Path)

			resp := dispatch(req)
			respPayload, _ := json.Marshal(resp)
			if err := writeFrame(conn, typeResponse, respPayload); err != nil {
				log.Printf("[go:api] failed to send response: %v", err)
			}
		}
	}
}
