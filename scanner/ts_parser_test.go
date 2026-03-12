package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTSFile(t *testing.T) {
	src := `
// @Route(GET /api/products/:id)
// @Validate( zod )
// @Auth(roles: ["user", "guest"])
export async function getProduct(id: string) {}

// @Page(/dashboard)
// @Auth(roles: ["user"])
export default function DashboardPage() {}
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "products.ts")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := ParseTSFiles(tmpDir, "node:products")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	r0 := routes[0]
	if r0.Method != "GET" || r0.Path != "/api/products/:id" || r0.Type != "api" {
		t.Errorf("unexpected route: %+v", r0)
	}
	if len(r0.AuthRoles) != 2 {
		t.Errorf("expected 2 roles, got %v", r0.AuthRoles)
	}

	r1 := routes[1]
	if r1.Method != "GET" || r1.Path != "/dashboard" || r1.Type != "page" {
		t.Errorf("unexpected page route: %+v", r1)
	}
}
