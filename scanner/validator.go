package scanner

import (
	"fmt"
	"os"
	"strings"
)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true,
	"PATCH": true, "DELETE": true, "HEAD": true, "OPTIONS": true,
}

// Validate checks each route for correctness and returns semantic errors.
// Each AnnotationError includes the File and Line from the originating Route.
// The second return value contains non-fatal warnings.
func Validate(routes []Route) ([]AnnotationError, []string) {
	var errs []AnnotationError
	var warnings []string
	seen := map[string]bool{}

	for _, r := range routes {
		if !validMethods[r.Method] {
			errs = append(errs, AnnotationError{
				File:    r.File,
				Line:    r.Line,
				Message: fmt.Sprintf("unknown HTTP method %q on route %s", r.Method, r.Path),
			})
		}
		if !strings.HasPrefix(r.Path, "/") {
			errs = append(errs, AnnotationError{
				File:    r.File,
				Line:    r.Line,
				Message: fmt.Sprintf("route path %q must start with /", r.Path),
			})
		}
		key := r.Method + " " + r.Path
		if seen[key] {
			errs = append(errs, AnnotationError{
				File:    r.File,
				Line:    r.Line,
				Message: fmt.Sprintf("duplicate route %s %s", r.Method, r.Path),
			})
		}
		seen[key] = true

		// Semantic validation (may return multiple errors per route)
		for _, err := range validateRoute(r) {
			errs = append(errs, AnnotationError{
				File:    r.File,
				Line:    r.Line,
				Message: err.Error(),
			})
		}
	}

	// Warnings for GET routes without @Response (future requirement)
	for _, r := range routes {
		if r.Method == "GET" && r.Type != "page" {
			// TODO(#165): emit warning when @Response annotation is implemented
			_ = r
		}
	}

	return errs, warnings
}

// validateRoute checks that a route has the required annotations for its method.
// Returns nil if valid, or a slice of errors describing what is missing.
func validateRoute(r Route) []error {
	var errs []error

	switch r.Method {
	case "POST", "PUT", "PATCH":
		if r.Validate == "" {
			errs = append(errs, fmt.Errorf("route %s %s: POST/PUT/PATCH routes must have @Validate annotation", r.Method, r.Path))
		}
	}

	if len(r.AuthRoles) == 0 && r.Type != "page" {
		errs = append(errs, fmt.Errorf("route %s %s: all routes must have @Auth annotation with roles", r.Method, r.Path))
	}

	return errs
}

// PrintWarnings outputs each warning string to stderr prefixed with "warning: ".
func PrintWarnings(warnings []string) {
	for _, w := range warnings {
		_, _ = fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
}
