package scanner

import (
	"encoding/json"
	"fmt"
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
//	pyDir       — directory containing Python backend files with @Route annotations (may be empty)
//	frontendDir — directory containing React TSX files with @Page/@Auth annotations (#16)
//	infraDir    — directory containing infrastructure resource definitions (may be empty)
//	outputPath  — file path where route_map.json will be written
func Generate(goDir, tsDir, pyDir, frontendDir, outputPath string) ([]AnnotationError, error) {
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

	if pyDir != "" {
		routes, errs := ParsePyFiles(pyDir, "python:"+filepath.Base(pyDir))
		allRoutes = append(allRoutes, routes...)
		allErrs = append(allErrs, errs...)
	}

	// #16: scan React TSX frontend pages.
	if frontendDir != "" {
		routes, errs := ParseTSXFiles(frontendDir, "node:ssr")
		allRoutes = append(allRoutes, routes...)
		allErrs = append(allErrs, errs...)
	}

	validErrs, warnings := Validate(allRoutes)
	allErrs = append(allErrs, validErrs...)

	// Print warnings to stderr but do not block route_map.json generation.
	PrintWarnings(warnings)

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

// GenerateInfra scans infrastructure directories and writes infra_map.json.
// Ignores errors from individual parsers and reports all annotation errors.
func GenerateInfra(infraDir, outputPath string) ([]AnnotationError, error) {
	if infraDir == "" {
		return nil, nil // infra scanning is optional
	}

	resources, errs := ParseInfraFiles(infraDir)

	infraMap := InfraResourceMap{
		Resources: resources,
	}

	data, err := json.MarshalIndent(infraMap, "", "  ")
	if err != nil {
		return errs, fmt.Errorf("marshal infra_map.json: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return errs, fmt.Errorf("create infra_map output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return errs, fmt.Errorf("write infra_map.json: %w", err)
	}

	return errs, nil
}
