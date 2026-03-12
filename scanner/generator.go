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
//
// Parameters:
//
//	goDir       — directory containing Go source files with @Route annotations (may be empty)
//	tsDir       — directory containing TypeScript/JS backend files (may be empty)
//	frontendDir — directory containing React TSX files with @Page/@Auth annotations (#16)
//	outputPath  — file path where route_map.json will be written
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

	// #16: scan React TSX frontend pages.
	if frontendDir != "" {
		routes, errs := ParseTSXFiles(frontendDir, "node:ssr")
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
