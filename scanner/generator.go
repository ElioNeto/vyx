package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// RouteMap is the top-level structure written to route_map.json.
type RouteMap struct {
	Routes []Route `json:"routes"`
}

// Generate collects routes from the given backend and frontend directories
// and writes the result to outputPath (typically route_map.json).
func Generate(goDir, tsDir, frontendDir, outputPath string) ([]AnnotationError, error) {
	var allRoutes []Route
	var allErrs []AnnotationError

	if goDir != "" {
		routes, errs := ParseGoFiles(goDir, "go:"+filepath.Base(goDir))
		allRoutes = append(allRoutes, routes...)
		allErrs = append(allErrs, errs...)
	}

	if tsDir != "" {
		routes, errs := ParseTSFiles(tsDir, "node:"+filepath.Base(tsDir))
		allRoutes = append(allRoutes, routes...)
		allErrs = append(allErrs, errs...)
	}

	if frontendDir != "" {
		routes, errs := ParseTSFiles(frontendDir, "node:frontend")
		allRoutes = append(allRoutes, routes...)
		allErrs = append(allErrs, errs...)
	}

	validErrs := Validate(allRoutes)
	allErrs = append(allErrs, validErrs...)

	if len(allErrs) > 0 {
		return allErrs, nil
	}

	routeMap := RouteMap{Routes: allRoutes}
	data, err := json.MarshalIndent(routeMap, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, err
	}

	return nil, os.WriteFile(outputPath, data, 0644)
}
