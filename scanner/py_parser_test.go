package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePyFile(t *testing.T) {
	src := `package handler

# @Route(POST /api/orders)
# @Validate(pydantic)
# @Auth(roles: ["user"])
class OrderInput(BaseModel):
    product_id: str
    quantity: int

def create_order(data: OrderInput):
    return {"order_id": 456}

# @Route(GET /api/orders)
def list_orders():
    return []
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "handler.py")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := ParsePyFiles(tmpDir, "python:api")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	r0 := routes[0]
	if r0.Method != "POST" || r0.Path != "/api/orders" {
		t.Errorf("unexpected route: %+v", r0)
	}
	if r0.Validate != "pydantic" {
		t.Errorf("unexpected validate: %q", r0.Validate)
	}
	if len(r0.AuthRoles) != 1 || r0.AuthRoles[0] != "user" {
		t.Errorf("unexpected auth roles: %v", r0.AuthRoles)
	}

	r1 := routes[1]
	if r1.Method != "GET" || r1.Path != "/api/orders" {
		t.Errorf("unexpected route: %+v", r1)
	}
}

func TestParsePyFile_FileOpenError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "noaccess.py")
	os.WriteFile(path, []byte("# test"), 0000)
	defer os.Chmod(path, 0644)

	routes, errs := parsePyFile(path, "python:bad")
	assert.Empty(t, routes)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Message, "cannot open file")
}