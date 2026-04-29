package gateway_test

import (
	"encoding/json"
	"errors"
	"testing"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

func TestValidationError_Error(t *testing.T) {
	ve := &dgw.ValidationError{
		Details: []dgw.ValidationDetail{
			{Field: "name", Message: "required"},
			{Field: "age", Message: "must be numeric"},
		},
	}

	errStr := ve.Error()
	if errStr == "" {
		t.Error("Error() should not return empty string")
	}
	if len(ve.Details) != 2 {
		t.Errorf("Details should have 2 entries, got %d", len(ve.Details))
	}
}

func TestValidationError_MarshalJSON(t *testing.T) {
	ve := &dgw.ValidationError{
		Details: []dgw.ValidationDetail{
			{Field: "email", Message: "invalid format"},
		},
	}

	data, err := json.Marshal(ve)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify structure
	if _, ok := decoded["error"]; !ok {
		t.Error("missing 'error' field in JSON")
	}
}

func TestValidationError_Is(t *testing.T) {
	ve := &dgw.ValidationError{}
	
	// Test Is method - should return true when target is ErrSchemaValidation
	if !ve.Is(dgw.ErrSchemaValidation) {
		t.Error("Is should return true for ErrSchemaValidation")
	}
	
	// Test with different error type
	if ve.Is(errors.New("other")) {
		t.Error("Is should return false for different error type")
	}
}

func TestErrRouteNotFound(t *testing.T) {
	err := dgw.ErrRouteNotFound
	if err == nil {
		t.Fatal("ErrRouteNotFound should not be nil")
	}
	
	// Verify it's an error
	var _ error = err
}

func TestErrUnauthorized(t *testing.T) {
	err := dgw.ErrUnauthorized
	if err == nil {
		t.Fatal("ErrUnauthorized should not be nil")
	}
}

func TestErrForbidden(t *testing.T) {
	err := dgw.ErrForbidden
	if err == nil {
		t.Fatal("ErrForbidden should not be nil")
	}
}

func TestErrSchemaValidation(t *testing.T) {
	err := dgw.ErrSchemaValidation
	if err == nil {
		t.Fatal("ErrSchemaValidation should not be nil")
	}
}

func TestErrPayloadTooLarge(t *testing.T) {
	err := dgw.ErrPayloadTooLarge
	if err == nil {
		t.Fatal("ErrPayloadTooLarge should not be nil")
	}
}

func TestErrUpstreamTimeout(t *testing.T) {
	err := dgw.ErrUpstreamTimeout
	if err == nil {
		t.Fatal("ErrUpstreamTimeout should not be nil")
	}
}

func TestValidationDetail_Struct(t *testing.T) {
	detail := dgw.ValidationDetail{
		Field:   "username",
		Message: "too short",
	}
	
	if detail.Field != "username" {
		t.Errorf("Field = %q, want %q", detail.Field, "username")
	}
	if detail.Message != "too short" {
		t.Errorf("Message = %q, want %q", detail.Message, "too short")
	}
}

func TestGatewayRequest_Struct(t *testing.T) {
	req := &dgw.GatewayRequest{
		Method: "POST",
		Path:   "/api/test",
		Headers: map[string]string{"Content-Type": "application/json"},
		Query:   map[string]string{"id": "123"},
		Body:    []byte(`{"test":true}`),
	}
	
	if req.Method != "POST" {
		t.Errorf("Method = %q, want %q", req.Method, "POST")
	}
	if len(req.Body) == 0 {
		t.Error("Body should not be empty")
	}
}

func TestGatewayResponse_Struct(t *testing.T) {
	resp := &dgw.GatewayResponse{
		StatusCode:     200,
		Body:           []byte(`{"ok":true}`),
		Headers:        map[string]string{"X-Custom": "value"},
		CorrelationID: "corr-123",
	}
	
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if resp.CorrelationID != "corr-123" {
		t.Errorf("CorrelationID = %q, want %q", resp.CorrelationID, "corr-123")
	}
}

func TestClaims_Struct(t *testing.T) {
	claims := &dgw.Claims{
		UserID: "user123",
		Roles:  []string{"admin", "user"},
	}
	
	if claims.UserID != "user123" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user123")
	}
	if len(claims.Roles) != 2 {
		t.Errorf("len(Roles) = %d, want 2", len(claims.Roles))
	}
}

func TestWorkerResponse_Struct(t *testing.T) {
	resp := &dgw.WorkerResponse{
		StatusCode:    200,
		Body:          []byte(`{"data":"test"}`),
		CorrelationID: "worker-corr",
	}
	
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if resp.CorrelationID != "worker-corr" {
		t.Errorf("CorrelationID = %q, want %q", resp.CorrelationID, "worker-corr")
	}
}

// Test ValidationError with empty details (lines 34-47)
func TestValidationError_EmptyDetails(t *testing.T) {
	ve := &dgw.ValidationError{
		Details: []dgw.ValidationDetail{},
	}
	
	errStr := ve.Error()
	if errStr == "" {
		t.Error("Error() should not return empty string for empty details")
	}
	
	// Test Is method with nil target
	if ve.Is(nil) {
		t.Error("Is(nil) should return false")
	}
}

// Test ValidationError Is with different error types (lines 34-47)
func TestValidationError_IsDifferentTypes(t *testing.T) {
	ve := &dgw.ValidationError{}
	
	// Test with ErrSchemaValidation
	if !ve.Is(dgw.ErrSchemaValidation) {
		t.Error("Is(ErrSchemaValidation) should return true")
	}
	
	// Test with different error
	otherErr := errors.New("other error")
	if ve.Is(otherErr) {
		t.Error("Is(otherErr) should return false")
	}
}

// Test GatewayRequest with empty fields (lines 34-47)
func TestGatewayRequest_EmptyFields(t *testing.T) {
	req := &dgw.GatewayRequest{
		Method: "",
		Path:   "",
	}
	
	if req.Method != "" {
		t.Errorf("expected empty method")
	}
	if req.Path != "" {
		t.Errorf("expected empty path")
	}
}

// Test GatewayResponse with empty fields (lines 34-47)
func TestGatewayResponse_EmptyFields(t *testing.T) {
	resp := &dgw.GatewayResponse{
		StatusCode: 0,
		Body:       nil,
	}
	
	if resp.StatusCode != 0 {
		t.Errorf("expected zero status code")
	}
	if resp.Body != nil {
		t.Errorf("expected nil body")
	}
}
