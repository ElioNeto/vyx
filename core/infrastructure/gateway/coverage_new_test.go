package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// TestJWTValidatorValidate_ValidToken tests JWT validation with valid token
func TestJWTValidatorValidate_ValidToken(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)

	// Create a valid token
	claims := jwt.MapClaims{
		"sub":   "user123",
		"roles": []string{"admin", "user"},
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	result, err := validator.Validate(tokenStr)
	if err != nil {
		t.Errorf("Validate() returned error: %v", err)
	}
	if result.UserID != "user123" {
		t.Errorf("UserID = %v, want %v", result.UserID, "user123")
	}
	if len(result.Roles) != 2 || result.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin, user]", result.Roles)
	}
}

// TestJWTValidatorValidate_InvalidToken tests JWT validation with invalid token
func TestJWTValidatorValidate_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)

	// Test with malformed token
	_, err := validator.Validate("not-a-valid-jwt-token")
	if err == nil {
		t.Error("Expected error for malformed token")
	}
}

// TestJWTValidatorValidate_ExpiredToken tests JWT validation with expired token
func TestJWTValidatorValidate_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)

	// Create expired token
	claims := jwt.MapClaims{
		"sub":   "user123",
		"roles": []string{"user"},
		"exp":   float64(time.Now().Add(-time.Hour).Unix()), // Expired 1 hour ago
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(secret)

	_, err := validator.Validate(tokenStr)
	if err == nil {
		t.Error("Expected error for expired token")
	}
}

// TestJWTValidatorValidate_EmptyToken tests JWT validation with empty token
func TestJWTValidatorValidate_EmptyToken(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)

	_, err := validator.Validate("")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

// TestSchemaValidatorWarmUp_EmptyDir tests WarmUp with empty directory
func TestSchemaValidatorWarmUp_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewSchemaValidator(tmpDir)

	err := validator.WarmUp()
	if err != nil {
		t.Errorf("WarmUp() returned error for empty directory: %v", err)
	}
}

// TestSchemaValidatorWarmUp_ValidSchemas tests WarmUp with valid schemas
func TestSchemaValidatorWarmUp_ValidSchemas(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid JSON schema
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"name": map[string]string{"type": "string"}},
		"required":   []string{"name"},
	}
	schemaJSON, _ := json.Marshal(schema)

	if err := os.WriteFile(filepath.Join(tmpDir, "test_schema.json"), schemaJSON, 0644); err != nil {
		t.Fatalf("Failed to write schema file: %v", err)
	}

	validator := NewSchemaValidator(tmpDir)
	err := validator.WarmUp()
	if err != nil {
		t.Errorf("WarmUp() returned error for valid schema: %v", err)
	}
}

// TestSchemaValidatorValidate_SchemaNotFound tests Validate with non-existent schema
func TestSchemaValidatorValidate_SchemaNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewSchemaValidator(tmpDir)

	body := []byte(`{}`)
	err := validator.Validate("non_existent", body)
	if err == nil {
		t.Error("Expected error for non-existent schema")
	}
}

// TestSchemaValidatorValidate_InvalidJSONBody tests Validate with invalid JSON body
func TestSchemaValidatorValidate_InvalidJSONBody(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid schema
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"name": map[string]string{"type": "string"}},
		"required":   []string{"name"},
	}
	schemaJSON, _ := json.Marshal(schema)

	if err := os.WriteFile(filepath.Join(tmpDir, "user.json"), schemaJSON, 0644); err != nil {
		t.Fatalf("Failed to write schema file: %v", err)
	}

	validator := NewSchemaValidator(tmpDir)
	if err := validator.WarmUp(); err != nil {
		t.Fatalf("WarmUp failed: %v", err)
	}

	// Test with invalid JSON body
	body := []byte(`not valid json`)
	err := validator.Validate("user", body)
	if err == nil {
		t.Error("Expected error for invalid JSON body")
	}
}

// TestNewServer_WithTLSConfig tests New function with TLS configuration
func TestNewServer_WithTLSConfig(t *testing.T) {
	log := zap.NewNop()
	cfg := DefaultConfig()
	cfg.TLSCertFile = "cert.pem"
	cfg.TLSKeyFile = "key.pem"

	rm := dgw.NewRouteMap(nil)
	transport := &mockTransport{}
	jwtVal := &mockJWT{}
	schemaVal := &mockSchema{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwtVal,
		Schema:    schemaVal,
		Timeout:   time.Second,
		Log:       log,
	})

	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), log)
	if server == nil {
		t.Fatal("New() returned nil")
	}
}

// TestHandle_RouteNotFound tests handle function with route not found
func TestHandle_RouteNotFound(t *testing.T) {
	log := zap.NewNop()
	cfg := DefaultConfig()

	// Create route map with no routes
	rm := dgw.NewRouteMap(nil)

	transport := &mockTransport{}
	jwtVal := &mockJWT{}
	schemaVal := &mockSchema{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwtVal,
		Schema:    schemaVal,
		Timeout:   time.Second,
		Log:       log,
	})

	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), log)

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/nonexistent", nil)
	w := httptest.NewRecorder()

	// Call handle directly (it's unexported, but we're in the same package)
	server.handle(w, req)

	// Should return 404 Not Found
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestHandle_WithValidRoute tests handle function with route found
func TestHandle_WithValidRoute(t *testing.T) {
	log := zap.NewNop()
	cfg := DefaultConfig()

	// Create route map with a route
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/test"},
	})

	// Transport that returns a response
	transport := &mockTransportResp{}
	jwtVal := &mockJWTValid{}
	schemaVal := &mockSchemaValid{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwtVal,
		Schema:    schemaVal,
		Timeout:   time.Second,
		Log:       log,
	})

	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), log)

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	server.handle(w, req)

	// Should not return 404
	if w.Code == http.StatusNotFound {
		t.Errorf("Should not return 404 for valid route, got %d", w.Code)
	}
}

// TestHandle_WithBody tests handle function with request body
func TestHandle_WithBody(t *testing.T) {
	log := zap.NewNop()
	cfg := DefaultConfig()

	// Create route map with POST route
	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "POST", Path: "/api/data"},
	})

	// Transport that returns a response
	transport := &mockTransportResp{}
	jwtVal := &mockJWTValid{}
	schemaVal := &mockSchemaValid{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwtVal,
		Schema:    schemaVal,
		Timeout:   time.Second,
		Log:       log,
	})

	server := New(cfg, dispatcher, apgw.NewRateLimiter(100, 100, time.Minute), log)

	// Create a POST request with body
	body := strings.NewReader(`{"name": "test"}`)
	req := httptest.NewRequest("POST", "http://example.com/api/data", body)
	w := httptest.NewRecorder()

	server.handle(w, req)

	// Should not return 404 or error
	if w.Code == http.StatusNotFound {
		t.Errorf("Should not return 404 for valid route, got %d", w.Code)
	}
}

// TestProxy_RouteNotFound tests proxy with no matching route
func TestProxy_RouteNotFound(t *testing.T) {
	log := zap.NewNop()

	// Empty route map
	rm := dgw.NewRouteMap(nil)

	transport := &mockTransport{}
	jwtVal := &mockJWTValid{}

	proxy := newWSProxy(rm, transport, jwtVal, log, time.Second)

	// Create a websocket upgrade request
	req := httptest.NewRequest("GET", "http://example.com/ws/nonexistent", nil)
	req.Header.Set("Upgrade", "websocket")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	// Should return 404
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestProxy_WithAuth tests proxy with authentication required
func TestProxy_WithAuth(t *testing.T) {
	log := zap.NewNop()

	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "WS", Path: "/ws/test", AuthRoles: []string{"admin"}},
	})

	transport := &mockTransport{}
	// mockJWTInvalid returns error for any token
	jwtVal := &mockJWTInvalid{}

	proxy := newWSProxy(rm, transport, jwtVal, log, time.Second)

	// Create a websocket request without auth
	req := httptest.NewRequest("GET", "http://example.com/ws/test", nil)
	req.Header.Set("Upgrade", "websocket")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	// Should return 401 Unauthorized (no auth provided)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestProxy_WithForbiddenAuth tests proxy with insufficient roles
func TestProxy_WithForbiddenAuth(t *testing.T) {
	log := zap.NewNop()

	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "WS", Path: "/ws/test", AuthRoles: []string{"admin"}},
	})

	transport := &mockTransport{}
	// mockJWTNoRole returns claims with "user" role (not "admin")
	jwtVal := &mockJWTNoRole{}

	proxy := newWSProxy(rm, transport, jwtVal, log, time.Second)

	// Create a websocket request with auth but insufficient role
	req := httptest.NewRequest("GET", "http://example.com/ws/test", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Authorization", "Bearer some-valid-token")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	// Should return 403 Forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// TestToValidationError_Indirect tests toValidationError indirectly through Validate
func TestToValidationError_Indirect(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a schema that will fail validation
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"name": map[string]string{"type": "string"}},
		"required":   []string{"name"},
	}
	schemaJSON, _ := json.Marshal(schema)

	if err := os.WriteFile(filepath.Join(tmpDir, "user.json"), schemaJSON, 0644); err != nil {
		t.Fatalf("Failed to write schema file: %v", err)
	}

	validator := NewSchemaValidator(tmpDir)
	if err := validator.WarmUp(); err != nil {
		t.Fatalf("WarmUp failed: %v", err)
	}

	// Validate with missing required field - this will call toValidationError
	body := []byte(`{}`)
	err := validator.Validate("user", body)
	if err == nil {
		t.Error("Expected validation error")
	}

	// Verify it's a ValidationError (which is created by toValidationError)
	if _, ok := err.(*dgw.ValidationError); !ok {
		t.Errorf("Expected *dgw.ValidationError, got %T", err)
	}
}
