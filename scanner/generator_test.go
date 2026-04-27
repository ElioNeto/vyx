package scanner

import (
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