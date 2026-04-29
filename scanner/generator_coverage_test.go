package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate_JsonMarshalSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}

	goSrc := "// @Route(GET /api/test)\nfunc handler() {}\n"
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

func TestGenerate_CreateNestedOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	goDir := filepath.Join(tmpDir, "go")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}

	goSrc := "// @Route(GET /api/health)\nfunc health() {}\n"
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
