package scanner

import (
	"strings"
	"testing"
)

func TestValidate_InvalidMethod(t *testing.T) {
	routes := []Route{
		{Path: "/api/test", Method: "INVALID", File: "handler.go", Line: 10, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
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
		{Path: "api/missing-slash", Method: "GET", File: "routes.ts", Line: 42, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
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
		{Path: "/api/users", Method: "GET", File: "users.go", Line: 5, AuthRoles: []string{"admin"}},
		{Path: "/api/users", Method: "GET", File: "users.go", Line: 20, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
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
		{Path: "/api/products", Method: "GET", File: "products.go", Line: 8, AuthRoles: []string{"admin"}},
		{Path: "/api/products", Method: "POST", File: "products.go", Line: 15, AuthRoles: []string{"admin"}, Validate: "ProductSchema"},
	}
	errs, _ := Validate(routes)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_ErrorMessage_ContainsLocation(t *testing.T) {
	routes := []Route{
		{Path: "/api/test", Method: "BOGUS", File: "svc.go", Line: 7, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
	if len(errs) == 0 {
		t.Fatal("expected an error")
	}
	msg := errs[0].Error()
	expected := "svc.go:7:"
	if len(msg) < len(expected) || msg[:len(expected)] != expected {
		t.Errorf("expected error to start with %q, got %q", expected, msg)
	}
}

// --- New semantic validation tests ---

func TestValidate_MissingValidateOnPost(t *testing.T) {
	routes := []Route{
		{Path: "/api/users", Method: "POST", File: "handler.go", Line: 5, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Message, "@Validate") {
		t.Errorf("expected error to mention @Validate, got: %s", errs[0].Message)
	}
}

func TestValidate_MissingValidateOnPut(t *testing.T) {
	routes := []Route{
		{Path: "/api/users/:id", Method: "PUT", File: "handler.go", Line: 10, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Message, "@Validate") {
		t.Errorf("expected error to mention @Validate, got: %s", errs[0].Message)
	}
}

func TestValidate_MissingValidateOnPatch(t *testing.T) {
	routes := []Route{
		{Path: "/api/users/:id", Method: "PATCH", File: "handler.go", Line: 15, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Message, "@Validate") {
		t.Errorf("expected error to mention @Validate, got: %s", errs[0].Message)
	}
}

func TestValidate_ValidateOnGetOk(t *testing.T) {
	// GET routes are not required to have @Validate.
	routes := []Route{
		{Path: "/api/users", Method: "GET", File: "handler.go", Line: 20, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
	if len(errs) != 0 {
		t.Errorf("expected no errors for GET without @Validate, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ValidateOnDeleteOk(t *testing.T) {
	// DELETE routes are not required to have @Validate.
	routes := []Route{
		{Path: "/api/users/:id", Method: "DELETE", File: "handler.go", Line: 25, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
	if len(errs) != 0 {
		t.Errorf("expected no errors for DELETE without @Validate, got %d: %v", len(errs), errs)
	}
}

func TestValidate_MissingAuthRoles(t *testing.T) {
	routes := []Route{
		{Path: "/api/users", Method: "GET", File: "handler.go", Line: 30},
	}
	errs, _ := Validate(routes)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Message, "@Auth") {
		t.Errorf("expected error to mention @Auth, got: %s", errs[0].Message)
	}
}

func TestValidate_PageRouteNoAuthRequired(t *testing.T) {
	// Page routes are exempt from @Auth requirement.
	routes := []Route{
		{Path: "/dashboard", Method: "GET", Type: "page", File: "pages.tsx", Line: 5},
	}
	errs, _ := Validate(routes)
	if len(errs) != 0 {
		t.Errorf("expected no errors for page route without @Auth, got %d: %v", len(errs), errs)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	// POST route without @Validate and without @Auth should produce two errors.
	routes := []Route{
		{Path: "/api/users", Method: "POST", File: "handler.go", Line: 35},
	}
	errs, _ := Validate(routes)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ValidRoutesWithAnnotations(t *testing.T) {
	routes := []Route{
		{Path: "/api/users", Method: "GET", File: "users.go", Line: 5, AuthRoles: []string{"admin"}},
		{Path: "/api/users", Method: "POST", File: "users.go", Line: 10, AuthRoles: []string{"admin"}, Validate: "CreateUserSchema"},
		{Path: "/api/users/:id", Method: "PUT", File: "users.go", Line: 15, AuthRoles: []string{"admin"}, Validate: "UpdateUserSchema"},
		{Path: "/api/users/:id", Method: "PATCH", File: "users.go", Line: 20, AuthRoles: []string{"admin"}, Validate: "PatchUserSchema"},
		{Path: "/api/users/:id", Method: "DELETE", File: "users.go", Line: 25, AuthRoles: []string{"admin"}},
	}
	errs, _ := Validate(routes)
	if len(errs) != 0 {
		t.Errorf("expected no errors for fully annotated routes, got %d: %v", len(errs), errs)
	}
}
