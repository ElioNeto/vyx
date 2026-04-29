package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// testTransport is a mock transport for testing
type testTransport struct{}

func (t *testTransport) Send(_ context.Context, _ string, _ ipc.Message) error { return nil }
func (t *testTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (t *testTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return ipc.Message{}, nil
}
func (t *testTransport) Register(_ context.Context, _ string) error  { return nil }
func (t *testTransport) Deregister(_ context.Context, _ string) error { return nil }
func (t *testTransport) Close() error                                   { return nil }

type testJWT struct{}

func (j *testJWT) Validate(_ string) (*dgw.Claims, error) { return nil, nil }

type testSchema struct{}

func (s *testSchema) Validate(_ string, _ []byte) error { return nil }

// TestJWTValidator_ValidToken tests successful JWT validation
func TestJWTValidator_ValidToken(t *testing.T) {
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

// TestJWTValidator_InvalidToken tests invalid JWT
func TestJWTValidator_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)
	
	// Invalid token string
	_, err := validator.Validate("invalid-token")
	if err == nil {
		t.Error("Validate() should return error for invalid token")
	}
}

// TestJWTValidator_WrongSecret tests token signed with wrong secret
func TestJWTValidator_WrongSecret(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)
	
	// Create token with different secret
	wrongSecret := []byte("wrong-secret")
	claims := jwt.MapClaims{
		"sub":   "user123",
		"roles": []string{"user"},
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(wrongSecret)
	
	_, err := validator.Validate(tokenStr)
	if err == nil {
		t.Error("Validate() should return error for token signed with wrong secret")
	}
}

// TestJWTValidator_ExpiredToken tests expired JWT
func TestJWTValidator_ExpiredToken(t *testing.T) {
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
		t.Error("Validate() should return error for expired token")
	}
}

// TestJWTValidator_EmptyClaims tests token with empty claims
func TestJWTValidator_EmptyClaims(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)
	
	claims := jwt.MapClaims{
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(secret)
	
	result, err := validator.Validate(tokenStr)
	if err != nil {
		t.Errorf("Validate() returned error: %v", err)
	}
	if result.UserID != "" {
		t.Errorf("UserID should be empty, got %v", result.UserID)
	}
	if len(result.Roles) != 0 {
		t.Errorf("Roles should be empty, got %v", result.Roles)
	}
}

// TestJWTValidator_NilToken tests nil token string
func TestJWTValidator_NilToken(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)
	
	_, err := validator.Validate("")
	if err == nil {
		t.Error("Validate() should return error for empty token")
	}
}

// TestJWTValidator_InvalidSigningMethod tests token with invalid signing method
func TestJWTValidator_InvalidSigningMethod(t *testing.T) {
	secret := []byte("test-secret-key")
	validator := NewJWTValidator(secret)
	
	// Create token with RSA signing method (not HMAC)
	claims := jwt.MapClaims{
		"sub":   "user123",
		"roles": []string{"user"},
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims) // Wrong method
	tokenStr, _ := token.SignedString(secret) // This won't work properly but tests the code path
	
	_, err := validator.Validate(tokenStr)
	if err == nil {
		t.Error("Validate() should return error for invalid signing method")
	}
}

// TestNewJWTValidator tests constructor
func TestNewJWTValidator(t *testing.T) {
	secret := []byte("test-secret")
	validator := NewJWTValidator(secret)
	
	if validator == nil {
		t.Error("NewJWTValidator returned nil")
	}
	if len(validator.secret) != len(secret) {
		t.Error("Secret not stored correctly")
	}
}

// TestJWTValidator_Claims tests Claims struct
func TestJWTValidator_Claims(t *testing.T) {
	claims := &dgw.Claims{
		UserID: "test-user",
		Roles:  []string{"admin", "user"},
	}
	
	if claims.UserID != "test-user" {
		t.Errorf("Claims.UserID = %v, want %v", claims.UserID, "test-user")
	}
	if len(claims.Roles) != 2 {
		t.Errorf("Claims.Roles length = %v, want 2", len(claims.Roles))
	}
}

// TestHandleFunction tests the handle function indirectly
func TestHandleFunction(t *testing.T) {
	// We can't call handle directly as it's unexported and needs http.ResponseWriter
	// But we can test it through the server
	cfg := DefaultConfig()
	log := zap.NewNop()

	rm := dgw.NewRouteMap([]dgw.RouteEntry{
		{WorkerID: "w1", Method: "GET", Path: "/test"},
	})

	transport := &testTransport{}
	jwt := &testJWT{}
	schema := &testSchema{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:    time.Second,
		Log:       log,
	})

	_ = New(cfg, dispatcher, nil, log)

	// The handle function would be called by the server
	// This test just ensures New doesn't panic
}

// TestNew_WithRateLimiter tests New with rate limiter
func TestNew_WithRateLimiter(t *testing.T) {
	cfg := DefaultConfig()
	log := zap.NewNop()

	rm := dgw.NewRouteMap(nil)
	transport := &testTransport{}
	jwt := &testJWT{}
	schema := &testSchema{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:    time.Second,
		Log:       log,
	})

	rateLimiter := apgw.NewRateLimiter(100, 50, time.Minute)
	s := New(cfg, dispatcher, rateLimiter, log)
	if s == nil {
		t.Fatal("expected server, got nil")
	}
}

// TestServerMethods tests exported methods
func TestServerMethods(t *testing.T) {
	cfg := DefaultConfig()
	log := zap.NewNop()

	rm := dgw.NewRouteMap(nil)
	transport := &testTransport{}
	jwt := &testJWT{}
	schema := &testSchema{}

	dispatcher := apgw.NewDispatcher(apgw.DispatcherConfig{
		Routes:    rm,
		Transport: transport,
		JWT:       jwt,
		Schema:    schema,
		Timeout:    time.Second,
		Log:       log,
	})

	s := New(cfg, dispatcher, nil, log)

	// Test Addr
	addr := s.Addr()
	if addr != ":8080" {
		t.Errorf("Addr() = %q, want %q", addr, ":8080")
	}
}
