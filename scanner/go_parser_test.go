package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoFile(t *testing.T) {
	src := `package handler

// @Route(POST /api/users)
// @Validate(JsonSchema: "user_create")
// @Auth(roles: ["admin"])
func CreateUser() {}

// @Route(GET /api/users)
func ListUsers() {}
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	routes, errs := ParseGoFiles(tmpDir, "go:api")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	r0 := routes[0]
	if r0.Method != "POST" || r0.Path != "/api/users" {
		t.Errorf("unexpected route: %+v", r0)
	}
	if r0.Validate != `JsonSchema: "user_create"` {
		t.Errorf("unexpected validate: %q", r0.Validate)
	}
	if len(r0.AuthRoles) != 1 || r0.AuthRoles[0] != "admin" {
		t.Errorf("unexpected auth roles: %v", r0.AuthRoles)
	}

	r1 := routes[1]
	if r1.Method != "GET" || r1.Path != "/api/users" {
		t.Errorf("unexpected route: %+v", r1)
	}
}
