// Go worker for the ecommerce vyx example.
//
// This worker connects to the vyx core via Unix Domain Socket (UDS) on Unix
// or via Named Pipe on Windows, performs the handshake, and handles requests
// for product CRUD and cart management.
//
// Annotated routes (parsed at build time by `vyx build`):
//
// @Route(GET /api/products)
// @Auth(roles: ["guest", "user", "admin"])
//
// @Route(POST /api/products)
// @Validate(JsonSchema: "product")
// @Auth(roles: ["admin"])
//
// @Route(GET /api/products/:id)
// @Auth(roles: ["guest", "user", "admin"])
//
// @Route(PUT /api/products/:id)
// @Validate(JsonSchema: "product")
// @Auth(roles: ["admin"])
//
// @Route(DELETE /api/products/:id)
// @Auth(roles: ["admin"])
//
// @Route(GET /api/cart)
// @Auth(roles: ["user", "admin"])
//
// @Route(POST /api/cart/items)
// @Validate(JsonSchema: "cart-item")
// @Auth(roles: ["user", "admin"])
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// ─── IPC protocol types (mirrors core/domain/ipc/message.go) ─────────────────

const (
	typeRequest   = 0x01
	typeResponse  = 0x02
	typeHeartbeat = 0x03
	typeError     = 0x04
	typeHandshake = 0x05
)

type frame struct {
	Length  uint32
	MsgType uint8
	Payload []byte
}

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

// ─── In-memory data stores ──────────────────────────────────────────────────

type product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price"`
	Category    string  `json:"category,omitempty"`
	Stock       int     `json:"stock,omitempty"`
}

type cartItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type cart struct {
	Items []cartItem `json:"items"`
}

var (
	mu         sync.Mutex
	products   = make(map[string]product)
	carts      = make(map[string]*cart) // keyed by user ID (claims.sub)
	nextProdID = 1
)

func init() {
	// Seed some products for demonstration.
	products["1"] = product{ID: "1", Name: "Smartphone", Description: "Latest model smartphone", Price: 999.99, Category: "electronics", Stock: 50}
	products["2"] = product{ID: "2", Name: "T-Shirt", Description: "Cotton t-shirt", Price: 29.99, Category: "clothing", Stock: 200}
	products["3"] = product{ID: "3", Name: "Notebook", Description: "Premium notebook", Price: 49.99, Category: "books", Stock: 100}
	nextProdID = 4
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeFrame(conn net.Conn, msgType uint8, payload []byte) error {
	header := make([]byte, 5)
	binary.LittleEndian.PutUint32(header[0:4], uint32(len(payload)))
	header[4] = msgType
	_, err := conn.Write(append(header, payload...))
	return err
}

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

func dialSocket(socketPath string) (net.Conn, error) {
	if runtime.GOOS == "windows" {
		return dialNamedPipe(socketPath)
	}
	return net.Dial("unix", socketPath)
}

// matchPattern checks if path matches a route pattern like /api/products/:id
// and extracts the named parameters into the returned map.
func matchPattern(path, pattern string) (bool, map[string]string) {
	path = strings.TrimRight(path, "/")
	pattern = strings.TrimRight(pattern, "/")

	pathSegs := strings.Split(strings.Trim(path, "/"), "/")
	patSegs := strings.Split(strings.Trim(pattern, "/"), "/")

	if len(pathSegs) != len(patSegs) {
		return false, nil
	}

	params := make(map[string]string)
	for i := range patSegs {
		if strings.HasPrefix(patSegs[i], ":") {
			paramName := patSegs[i][1:]
			params[paramName] = pathSegs[i]
		} else if patSegs[i] != pathSegs[i] {
			return false, nil
		}
	}

	return true, params
}

// jsonResponse is a shorthand to build a JSON 200 response.
func jsonResponse(body any) response {
	return response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
}

func jsonError(status int, msg string) response {
	return response{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       map[string]string{"error": msg},
	}
}

// ─── Route handlers ───────────────────────────────────────────────────────────

// @Route(GET /api/products)
// @Auth(roles: ["guest", "user", "admin"])
func handleListProducts(req request) response {
	mu.Lock()
	defer mu.Unlock()

	prodList := make([]product, 0, len(products))
	for _, p := range products {
		prodList = append(prodList, p)
	}
	return jsonResponse(map[string]any{"products": prodList})
}

// @Route(POST /api/products)
// @Validate(JsonSchema: "product")
// @Auth(roles: ["admin"])
func handleCreateProduct(req request) response {
	var p product
	if err := json.Unmarshal(req.Body, &p); err != nil {
		return jsonError(400, "invalid product payload")
	}

	mu.Lock()
	id := strconv.Itoa(nextProdID)
	nextProdID++
	p.ID = id
	products[id] = p
	mu.Unlock()

	return response{
		StatusCode: 201,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       p,
	}
}

// @Route(GET /api/products/:id)
// @Auth(roles: ["guest", "user", "admin"])
func handleGetProduct(req request) response {
	id := req.Params["id"]
	mu.Lock()
	p, ok := products[id]
	mu.Unlock()
	if !ok {
		return jsonError(404, fmt.Sprintf("product %s not found", id))
	}
	return jsonResponse(p)
}

// @Route(PUT /api/products/:id)
// @Validate(JsonSchema: "product")
// @Auth(roles: ["admin"])
func handleUpdateProduct(req request) response {
	id := req.Params["id"]
	var updates product
	if err := json.Unmarshal(req.Body, &updates); err != nil {
		return jsonError(400, "invalid product payload")
	}

	mu.Lock()
	existing, ok := products[id]
	if !ok {
		mu.Unlock()
		return jsonError(404, fmt.Sprintf("product %s not found", id))
	}
	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.Description != "" {
		existing.Description = updates.Description
	}
	if updates.Price > 0 {
		existing.Price = updates.Price
	}
	if updates.Category != "" {
		existing.Category = updates.Category
	}
	if updates.Stock >= 0 {
		existing.Stock = updates.Stock
	}
	products[id] = existing
	mu.Unlock()

	return jsonResponse(existing)
}

// @Route(DELETE /api/products/:id)
// @Auth(roles: ["admin"])
func handleDeleteProduct(req request) response {
	id := req.Params["id"]
	mu.Lock()
	_, ok := products[id]
	if !ok {
		mu.Unlock()
		return jsonError(404, fmt.Sprintf("product %s not found", id))
	}
	delete(products, id)
	mu.Unlock()
	return jsonResponse(map[string]string{"status": "deleted", "id": id})
}

// @Route(GET /api/cart)
// @Auth(roles: ["user", "admin"])
func handleGetCart(req request) response {
	sub := ""
	if s, ok := req.Claims["sub"]; ok {
		sub = fmt.Sprintf("%v", s)
	}
	if sub == "" {
		return jsonError(401, "unauthorized")
	}

	mu.Lock()
	c, ok := carts[sub]
	if !ok {
		c = &cart{Items: []cartItem{}}
		carts[sub] = c
	}
	mu.Unlock()

	return jsonResponse(c)
}

// @Route(POST /api/cart/items)
// @Validate(JsonSchema: "cart-item")
// @Auth(roles: ["user", "admin"])
func handleAddCartItem(req request) response {
	sub := ""
	if s, ok := req.Claims["sub"]; ok {
		sub = fmt.Sprintf("%v", s)
	}
	if sub == "" {
		return jsonError(401, "unauthorized")
	}

	var item cartItem
	if err := json.Unmarshal(req.Body, &item); err != nil {
		return jsonError(400, "invalid cart item payload")
	}

	mu.Lock()
	// Verify product exists
	if _, exists := products[item.ProductID]; !exists {
		mu.Unlock()
		return jsonError(404, fmt.Sprintf("product %s not found", item.ProductID))
	}
	if _, exists := carts[sub]; !exists {
		carts[sub] = &cart{Items: []cartItem{}}
	}
	// If item already exists, increment quantity
	found := false
	for i, ci := range carts[sub].Items {
		if ci.ProductID == item.ProductID {
			carts[sub].Items[i].Quantity += item.Quantity
			found = true
			break
		}
	}
	if !found {
		carts[sub].Items = append(carts[sub].Items, item)
	}
	result := *carts[sub]
	mu.Unlock()

	return response{
		StatusCode: 201,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       result,
	}
}

// ─── Dispatcher ───────────────────────────────────────────────────────────────

func dispatch(req request) response {
	switch {
	case req.Method == "GET" && req.Path == "/api/products":
		return handleListProducts(req)
	case req.Method == "POST" && req.Path == "/api/products":
		return handleCreateProduct(req)
	case req.Method == "GET" && matchPatternBool(req.Path, "/api/products/:id"):
		return handleGetProduct(req)
	case req.Method == "PUT" && matchPatternBool(req.Path, "/api/products/:id"):
		return handleUpdateProduct(req)
	case req.Method == "DELETE" && matchPatternBool(req.Path, "/api/products/:id"):
		return handleDeleteProduct(req)
	case req.Method == "GET" && req.Path == "/api/cart":
		return handleGetCart(req)
	case req.Method == "POST" && req.Path == "/api/cart/items":
		return handleAddCartItem(req)
	default:
		return jsonError(404, "route not found")
	}
}

// matchPatternBool returns true if path matches the pattern.
func matchPatternBool(path, pattern string) bool {
	ok, _ := matchPattern(path, pattern)
	return ok
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

	// Handshake
	handshake := handshakePayload{
		WorkerID: "go:api",
		Capabilities: []capability{
			{Path: "/api/products", Method: "GET"},
			{Path: "/api/products", Method: "POST"},
			{Path: "/api/products/:id", Method: "GET"},
			{Path: "/api/products/:id", Method: "PUT"},
			{Path: "/api/products/:id", Method: "DELETE"},
			{Path: "/api/cart", Method: "GET"},
			{Path: "/api/cart/items", Method: "POST"},
		},
	}
	hsPayload, _ := json.Marshal(handshake)
	if err := writeFrame(conn, typeHandshake, hsPayload); err != nil {
		log.Fatalf("[go:api] handshake failed: %v", err)
	}
	log.Printf("[go:api] handshake sent")

	// Send an immediate heartbeat so the core marks this worker healthy
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
			log.Printf("[go:api] %s %s (claims.sub=%v)", req.Method, req.Path, req.Claims["sub"])

			resp := dispatch(req)
			respPayload, _ := json.Marshal(resp)
			if err := writeFrame(conn, typeResponse, respPayload); err != nil {
				log.Printf("[go:api] failed to send response: %v", err)
			}
		}
	}
}
