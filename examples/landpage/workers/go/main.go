// Go worker for the landpage vyx example.
//
// This worker connects to the vyx core via Unix Domain Socket (UDS) on Unix
// or via Named Pipe on Windows, performs the handshake, and handles requests.
//
// Annotated routes (parsed at build time by `vyx build`):
//
// @Route(GET /api/health)
// @Auth(roles: ["guest"])
//
// @Route(POST /api/contact)
// @Validate(JsonSchema: "contact")
// @Auth(roles: ["guest"])
//
// @Route(POST /api/newsletter)
// @Validate(JsonSchema: "newsletter")
// @Auth(roles: ["guest"])
//
// @Route(GET /api/newsletter/subscribers)
// @Auth(roles: ["admin"])
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
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

// ─── In-memory stores ──────────────────────────────────────────────────────────

var (
	mu                sync.Mutex
	contactMessages   []contactEntry
	newsletterEmails  []newsletterEntry
)

const maxContactEntries = 100

type contactEntry struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
	Subject string `json:"subject"`
}

type newsletterEntry struct {
	Email string `json:"email"`
	Name  string `json:"name"`
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

// ─── Route handlers ───────────────────────────────────────────────────────────

// @Route(GET /api/health)
// @Auth(roles: ["guest"])
func handleHealth(req request) response {
	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body: map[string]string{
			"status":    "ok",
			"service":   "landpage",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// @Route(POST /api/contact)
// @Validate(JsonSchema: "contact")
// @Auth(roles: ["guest"])
func handleContact(req request) response {
	var body struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Message string `json:"message"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "invalid body"},
		}
	}

	mu.Lock()
	if len(contactMessages) >= maxContactEntries {
		mu.Unlock()
		return response{
			StatusCode: 503,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "contact store full, try again later"},
		}
	}
	if body.Subject == "" {
		body.Subject = "general"
	}
	contactMessages = append(contactMessages, contactEntry{
		Name:    body.Name,
		Email:   body.Email,
		Message: body.Message,
		Subject: body.Subject,
	})
	mu.Unlock()

	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body: map[string]string{
			"message": "Thank you for your message! We'll get back to you soon.",
		},
	}
}

// @Route(POST /api/newsletter)
// @Validate(JsonSchema: "newsletter")
// @Auth(roles: ["guest"])
func handleNewsletter(req request) response {
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return response{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "invalid body"},
		}
	}

	mu.Lock()
	// Check for duplicate
	for _, entry := range newsletterEmails {
		if entry.Email == body.Email {
			mu.Unlock()
			return response{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]string{"message": "You are already subscribed!"},
			}
		}
	}
	newsletterEmails = append(newsletterEmails, newsletterEntry{
		Email: body.Email,
		Name:  body.Name,
	})
	mu.Unlock()

	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       map[string]string{"message": "Successfully subscribed to the newsletter!"},
	}
}

// @Route(GET /api/newsletter/subscribers)
// @Auth(roles: ["admin"])
func handleListSubscribers(req request) response {
	mu.Lock()
	subs := make([]newsletterEntry, len(newsletterEmails))
	copy(subs, newsletterEmails)
	mu.Unlock()

	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       map[string]any{"subscribers": subs, "total": len(subs)},
	}
}

// ─── Dispatcher ───────────────────────────────────────────────────────────────

func dispatch(req request) response {
	switch {
	case req.Method == "GET" && req.Path == "/api/health":
		return handleHealth(req)
	case req.Method == "POST" && req.Path == "/api/contact":
		return handleContact(req)
	case req.Method == "POST" && req.Path == "/api/newsletter":
		return handleNewsletter(req)
	case req.Method == "GET" && req.Path == "/api/newsletter/subscribers":
		return handleListSubscribers(req)
	default:
		return response{
			StatusCode: 404,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       map[string]string{"error": "route not found"},
		}
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
			{Path: "/api/health", Method: "GET"},
			{Path: "/api/contact", Method: "POST"},
			{Path: "/api/newsletter", Method: "POST"},
			{Path: "/api/newsletter/subscribers", Method: "GET"},
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
