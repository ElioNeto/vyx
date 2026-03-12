package scanner

import (
	"fmt"
	"strings"
)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true,
	"PATCH": true, "DELETE": true, "HEAD": true, "OPTIONS": true,
}

// Validate checks each route for correctness and returns semantic errors.
// Each AnnotationError includes the File and Line from the originating Route.
func Validate(routes []Route) []AnnotationError {
	var errs []AnnotationError
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
	}

	return errs
}
