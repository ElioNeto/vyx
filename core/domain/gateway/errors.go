package gateway

import (
	"encoding/json"
	"errors"
	"strings"
)

// Sentinel errors used by the gateway pipeline.
var (
	ErrRouteNotFound   = errors.New("route not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrSchemaValidation = errors.New("validation failed")
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrUpstreamTimeout = errors.New("upstream timeout")
)

// ValidationDetail holds a single field-level validation failure.
// It is part of the structured 400 response body (spec §4.3).
type ValidationDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError is a structured error returned by the SchemaValidator.
// It implements the error interface and marshals to the spec §4.3 format:
//
//	{"error":"validation_failed","details":[{"field":"email","message":"..."}]}
type ValidationError struct {
	Details []ValidationDetail `json:"details"`
}

func (e *ValidationError) Error() string {
	msgs := make([]string, 0, len(e.Details))
	for _, d := range e.Details {
		if d.Field != "" {
			msgs = append(msgs, d.Field+": "+d.Message)
		} else {
			msgs = append(msgs, d.Message)
		}
	}
	return "validation failed: " + strings.Join(msgs, "; ")
}

// MarshalJSON renders the spec-compliant response body.
func (e *ValidationError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Error   string             `json:"error"`
		Details []ValidationDetail `json:"details"`
	}{
		Error:   "validation_failed",
		Details: e.Details,
	})
}

// Is makes ValidationError unwrappable via errors.Is(err, ErrSchemaValidation).
func (e *ValidationError) Is(target error) bool {
	return target == ErrSchemaValidation
}
