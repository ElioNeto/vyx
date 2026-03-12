package gateway

import "errors"

var (
	// ErrRouteNotFound is returned when no route matches the request.
	ErrRouteNotFound = errors.New("gateway: route not found")
	// ErrUnauthorized is returned when the JWT is missing or invalid.
	ErrUnauthorized = errors.New("gateway: unauthorized")
	// ErrForbidden is returned when the caller lacks a required role.
	ErrForbidden = errors.New("gateway: forbidden")
	// ErrPayloadTooLarge is returned when the request body exceeds the limit.
	ErrPayloadTooLarge = errors.New("gateway: payload too large")
	// ErrSchemaValidation is returned when the request body fails JSON Schema validation.
	ErrSchemaValidation = errors.New("gateway: request body failed schema validation")
	// ErrUpstreamTimeout is returned when the worker does not respond in time.
	ErrUpstreamTimeout = errors.New("gateway: upstream worker timed out")
)
