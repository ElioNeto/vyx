package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseGoFile_MultipleAnnotations tests multiple annotations on same route.
func TestParseGoFile_MultipleAnnotations(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package main
// @Route(POST /api/data)
// @Validate(JsonSchema: "schema")
// @Auth(roles: ["admin", "user"])
func Handler() {}
`
	path := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parseGoFile(path, "go:test")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	r := routes[0]
	if r.Validate != `JsonSchema: "schema"` {
		t.Errorf("unexpected validate: %q", r.Validate)
	}
	if len(r.AuthRoles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(r.AuthRoles))
	}
}

// TestParsePyFile_ClassLineSkipped tests that class lines are skipped but pending route is flushed before.
func TestParsePyFile_ClassLineSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	// Class line only skips if there's no pending route
	// If there's a pending route, it should be processed before the class line
	src := `# @Route(GET /api/test)
class MyClass:
    pass
`
	path := filepath.Join(tmpDir, "handler.py")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parsePyFile(path, "python:test")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// The pending route should be flushed when non-comment line (class) is encountered
	if len(routes) != 1 {
		t.Fatalf("expected 1 route (flushed before class), got %d", len(routes))
	}
}

// TestParseTSFile_PageAnnotation tests @Page annotation in TS files.
func TestParseTSFile_PageAnnotation(t *testing.T) {
	tmpDir := t.TempDir()
	src := `// @Page(/home)
// @Auth(roles: ["user"])
function handler() {}
`
	path := filepath.Join(tmpDir, "page.ts")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parseTSFile(path, "node:test")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	r := routes[0]
	if r.Type != "page" {
		t.Errorf("expected type 'page', got %q", r.Type)
	}
	if r.Method != "GET" {
		t.Errorf("expected GET method, got %q", r.Method)
	}
}

// TestParseTSXFile_FlushOnNonAnnotation tests that pending page is flushed on non-annotation line.
func TestParseTSXFile_FlushOnNonAnnotation(t *testing.T) {
	tmpDir := t.TempDir()
	src := `// @Page(/about)
export default function About() { return <div>About</div> }
`
	path := filepath.Join(tmpDir, "About.tsx")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parseTSXFile(path, "node:ssr")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
}

// TestParseTSXFile_PendingAtEOF tests pending page at end of file.
func TestParseTSXFile_PendingAtEOF(t *testing.T) {
	tmpDir := t.TempDir()
	src := `// @Page(/contact)
`
	path := filepath.Join(tmpDir, "Contact.tsx")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parseTSXFile(path, "node:ssr")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
}

// TestParseRoleList tests various role list formats.
func TestParseRoleList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{`"admin", "user"`, []string{"admin", "user"}},
		{`"admin"`, []string{"admin"}},
		{`admin, user`, []string{"admin", "user"}}, // unquoted
		{`  `, nil},                                // empty/whitespace
		{`"a", "b", "c"`, []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		result := parseRoleList(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseRoleList(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i, r := range result {
			if r != tt.expected[i] {
				t.Errorf("parseRoleList(%q)[%d] = %q, want %q", tt.input, i, r, tt.expected[i])
			}
		}
	}
}

// TestParseTSXFile_EmptyPagePath tests empty @Page path error.
func TestParseTSXFile_EmptyPagePath(t *testing.T) {
	tmpDir := t.TempDir()
	src := `// @Page()
export default function Bad() { return null }
`
	path := filepath.Join(tmpDir, "Bad.tsx")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	_, errs := parseTSXFile(path, "node:ssr")
	if len(errs) == 0 {
		t.Fatal("expected error for empty @Page path")
	}
}

// TestParseGoFile_EOFWithPendingRoute tests pending route at EOF.
func TestParseGoFile_EOFWithPendingRoute(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package main
// @Route(GET /api/eof)
`
	path := filepath.Join(tmpDir, "eof.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parseGoFile(path, "go:test")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
}

// TestParsePyFile_EOFWithPendingRoute tests pending route at EOF.
func TestParsePyFile_EOFWithPendingRoute(t *testing.T) {
	tmpDir := t.TempDir()
	src := `# @Route(GET /api/eof)
`
	path := filepath.Join(tmpDir, "eof.py")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parsePyFile(path, "python:test")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
}

// TestGenerate_WithAllDirs tests Generate with Go, TS, Python, and frontend dirs.
func TestGenerate_WithAllDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Go route
	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}
	goSrc := `// @Route(GET /api/go)
func goHandler() {}
`
	if err := os.WriteFile(filepath.Join(goDir, "handler.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Create TS route
	tsDir := filepath.Join(tmpDir, "ts")
	if err := os.MkdirAll(tsDir, 0755); err != nil {
		t.Fatal(err)
	}
	tsSrc := `// @Route(POST /api/ts)
function tsHandler() {}
`
	if err := os.WriteFile(filepath.Join(tsDir, "handler.ts"), []byte(tsSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Python route
	pyDir := filepath.Join(tmpDir, "py")
	if err := os.MkdirAll(pyDir, 0755); err != nil {
		t.Fatal(err)
	}
	pySrc := `# @Route(GET /api/py)
def pyHandler():
    pass
`
	if err := os.WriteFile(filepath.Join(pyDir, "handler.py"), []byte(pySrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Create TSX frontend page
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatal(err)
	}
	tsxSrc := `// @Page(/home)
export default function Home() { return <div>Home</div> }
`
	if err := os.WriteFile(filepath.Join(frontendDir, "Home.tsx"), []byte(tsxSrc), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(tmpDir, "route_map.json")
	errs, err := Generate(goDir, tsDir, pyDir, frontendDir, outputPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"routes"`) {
		t.Fatal("expected routes in output")
	}
}

// TestBuildRoute_Valid tests buildRoute with valid annotation.
func TestBuildRoute_Valid(t *testing.T) {
	route, err := buildRoute("test.go", 1, "@Route(GET /api/health)", "", "", "go:test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.Method != "GET" || route.Path != "/api/health" {
		t.Errorf("unexpected route: %+v", route)
	}
}

// TestBuildPyRoute_Valid tests buildPyRoute with valid annotation.
func TestBuildPyRoute_Valid(t *testing.T) {
	route, err := buildPyRoute("test.py", 1, "@Route(POST /api/order)", "", "", "python:test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.Method != "POST" || route.Path != "/api/order" {
		t.Errorf("unexpected route: %+v", route)
	}
}

// TestBuildTSRoute_ValidRoute tests buildTSRoute with valid @Route annotation.
func TestBuildTSRoute_ValidRoute(t *testing.T) {
	route, err := buildTSRoute("test.ts", 1, "@Route(GET /api/health)", "", "", "", "node:test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.Method != "GET" || route.Path != "/api/health" || route.Type != "api" {
		t.Errorf("unexpected route: %+v", route)
	}
}

// TestBuildTSRoute_ValidPage tests buildTSRoute with valid @Page annotation.
func TestBuildTSRoute_ValidPage(t *testing.T) {
	route, err := buildTSRoute("test.ts", 1, "", "@Page(/dashboard)", "", "", "node:test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.Method != "GET" || route.Path != "/dashboard" || route.Type != "page" {
		t.Errorf("unexpected route: %+v", route)
	}
}

// TestBuildTSRoute_NoAnnotation tests buildTSRoute with no route or page annotation.
func TestBuildTSRoute_NoAnnotation(t *testing.T) {
	_, err := buildTSRoute("test.ts", 1, "", "", "", "", "node:test")
	if err == nil {
		t.Fatal("expected error for no annotation")
	}
}

// TestBuildTSRoute_PageOnly tests buildTSRoute with only page annotation.
func TestBuildTSRoute_PageOnly(t *testing.T) {
	route, err := buildTSRoute("test.ts", 1, "", "@Page(/page)", "", "", "node:test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.Type != "page" {
		t.Errorf("expected type 'page', got %q", route.Type)
	}
	if route.Method != "GET" {
		t.Errorf("expected GET, got %q", route.Method)
	}
}

// TestBuildTSRoute_RouteOnly tests buildTSRoute with only route annotation.
func TestBuildTSRoute_RouteOnly(t *testing.T) {
	route, err := buildTSRoute("test.ts", 1, "@Route(GET /api/test)", "", "", "", "node:test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.Type != "api" {
		t.Errorf("expected type 'api', got %q", route.Type)
	}
}

// TestParseGoFile_WithValidateOnly tests that validate without route doesn't crash.
func TestParseGoFile_WithValidateOnly(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package main
// @Validate(JsonSchema: "test")
func Handler() {}
`
	path := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parseGoFile(path, "go:test")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// No route should be created since there's no @Route
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(routes))
	}
}

// TestParsePyFile_WithValidateOnly tests that validate without route doesn't crash.
func TestParsePyFile_WithValidateOnly(t *testing.T) {
	tmpDir := t.TempDir()
	src := `# @Validate(JsonSchema: "test")
def handler():
    pass
`
	path := filepath.Join(tmpDir, "handler.py")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parsePyFile(path, "python:test")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// No route should be created since there's no @Route
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(routes))
	}
}

// TestParseTSFile_WithValidateOnly tests that validate without route doesn't crash.
func TestParseTSFile_WithValidateOnly(t *testing.T) {
	tmpDir := t.TempDir()
	src := `// @Validate(JsonSchema: "test")
function handler() {}
`
	path := filepath.Join(tmpDir, "handler.ts")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := parseTSFile(path, "node:test")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// No route should be created since there's no @Route or @Page
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(routes))
	}
}
