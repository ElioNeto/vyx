package scanner

import "testing"

func TestValidate_InvalidMethod(t *testing.T) {
	routes := []Route{
		{Path: "/api/test", Method: "INVALID", File: "handler.go", Line: 10},
	}
	errs := Validate(routes)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].File != "handler.go" {
		t.Errorf("expected File %q, got %q", "handler.go", errs[0].File)
	}
	if errs[0].Line != 10 {
		t.Errorf("expected Line 10, got %d", errs[0].Line)
	}
}

func TestValidate_PathMissingSlash(t *testing.T) {
	routes := []Route{
		{Path: "api/missing-slash", Method: "GET", File: "routes.ts", Line: 42},
	}
	errs := Validate(routes)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].File != "routes.ts" {
		t.Errorf("expected File %q, got %q", "routes.ts", errs[0].File)
	}
	if errs[0].Line != 42 {
		t.Errorf("expected Line 42, got %d", errs[0].Line)
	}
}

func TestValidate_DuplicateRoute(t *testing.T) {
	routes := []Route{
		{Path: "/api/users", Method: "GET", File: "users.go", Line: 5},
		{Path: "/api/users", Method: "GET", File: "users.go", Line: 20},
	}
	errs := Validate(routes)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].File != "users.go" {
		t.Errorf("expected File %q, got %q", "users.go", errs[0].File)
	}
	if errs[0].Line != 20 {
		t.Errorf("expected Line 20, got %d", errs[0].Line)
	}
}

func TestValidate_ValidRoutes_NoErrors(t *testing.T) {
	routes := []Route{
		{Path: "/api/products", Method: "GET", File: "products.go", Line: 8},
		{Path: "/api/products", Method: "POST", File: "products.go", Line: 15},
	}
	errs := Validate(routes)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_ErrorMessage_ContainsLocation(t *testing.T) {
	routes := []Route{
		{Path: "/api/test", Method: "BOGUS", File: "svc.go", Line: 7},
	}
	errs := Validate(routes)
	if len(errs) == 0 {
		t.Fatal("expected an error")
	}
	msg := errs[0].Error()
	expected := "svc.go:7:"
	if len(msg) < len(expected) || msg[:len(expected)] != expected {
		t.Errorf("expected error to start with %q, got %q", expected, msg)
	}
}
