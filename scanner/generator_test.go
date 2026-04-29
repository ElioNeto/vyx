package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate_WithPythonDir(t *testing.T) {
	tmpDir := t.TempDir()

	pyDir := filepath.Join(tmpDir, "python")
	if err := os.MkdirAll(pyDir, 0755); err != nil {
		t.Fatal(err)
	}

	pySrc := `# @Route(POST /api/orders)
# @Validate(pydantic)
# @Auth(roles: ["user"])
class OrderInput(BaseModel):
    product_id: str
    quantity: int

def create_order(data: OrderInput):
    return {"order_id": 456}
`
	if err := os.WriteFile(filepath.Join(pyDir, "orders.py"), []byte(pySrc), 0644); err != nil {
		t.Fatal(err)
	}

	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}

	goSrc := `// @Route(GET /api/health)
func health() {}
`
	if err := os.WriteFile(filepath.Join(goDir, "health.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(tmpDir, "route_map.json")

	_, err := Generate(goDir, "", pyDir, "", outputPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if string(data) == "" {
		t.Fatal("route_map.json is empty")
	}

	t.Logf("route_map.json content:\n%s", string(data))
}

func TestGenerate_EmptyDirs(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "route_map.json")

	// All dirs empty - should return errors since no routes found
	errs, err := Generate("", "", "", "", outputPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// No routes found means errs may be empty or have errors
	// Just verify it doesn't crash
	t.Logf("Errors: %v", errs)
}

func TestGenerate_WriteFileSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}

	goSrc := `// @Route(GET /api/health)
func health() {}
`
	if err := os.WriteFile(filepath.Join(goDir, "health.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(tmpDir, "route_map.json")

	errs, err := Generate(goDir, "", "", "", outputPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Verify file was written
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("output file was not created")
	}
}

func TestGenerate_JsonMarshalSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}

	goSrc := `// @Route(GET /api/test)
func handler() {}
`
	if err := os.WriteFile(filepath.Join(goDir, "test.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(tmpDir, "route_map.json")
	errs, err := Generate(goDir, "", "", "", outputPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	var rm map[string]interface{}
	if err := json.Unmarshal(data, &rm); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}
}

func TestGenerate_NestedOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}

	goSrc := `// @Route(GET /api/health)
func health() {}
`
	if err := os.WriteFile(filepath.Join(goDir, "health.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Create nested output path - Generate should create parent dirs
	outputPath := filepath.Join(tmpDir, "deep", "nested", "route_map.json")

	errs, err := Generate(goDir, "", "", "", outputPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("output file was not created")
	}
}

func TestGenerate_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Invalid route syntax
	goSrc := `// @Route(INVALID /api/test)
func bad() {}
`
	if err := os.WriteFile(filepath.Join(goDir, "bad.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(tmpDir, "route_map.json")

	errs, err := Generate(goDir, "", "", "", outputPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Should have validation errors (invalid route syntax)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for invalid route syntax")
	}
}

func TestGenerate_MultipleRouteTypes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Go route
	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}

	goSrc := `// @Route(GET /api/test)
// @Auth(roles: ["admin"])
func testHandler() {}

// @Route(POST /api/data)
// @Validate(JsonSchema: "DataSchema")
func dataHandler() {}
`
	if err := os.WriteFile(filepath.Join(goDir, "test.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(tmpDir, "route_map.json")

	errs, err := Generate(goDir, "", "", "", outputPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Verify output is valid JSON with routes
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	var routeMap map[string]interface{}
	if err := json.Unmarshal(data, &routeMap); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	routes, ok := routeMap["routes"].([]interface{})
	if !ok || len(routes) == 0 {
		t.Fatal("expected routes in output")
	}
}
