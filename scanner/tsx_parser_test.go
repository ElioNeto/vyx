package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ElioNeto/vyx/scanner"
)

func writeTSX(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestParseTSXFiles_BasicPage(t *testing.T) {
	dir := t.TempDir()
	writeTSX(t, dir, "Home.tsx", `
// @Page(/)
export default function HomePage() {
  return <h1>Home</h1>
}
`)
	routes, errs := scanner.ParseTSXFiles(dir, "node:ssr")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	r := routes[0]
	if r.Path != "/" || r.Method != "GET" || r.Type != "page" || r.WorkerID != "node:ssr" {
		t.Errorf("unexpected route: %+v", r)
	}
}

func TestParseTSXFiles_PageWithAuth(t *testing.T) {
	dir := t.TempDir()
	writeTSX(t, dir, "Dashboard.tsx", `
// @Page(/dashboard)
// @Auth(roles: ["user", "admin"])
export default function DashboardPage() {
  return <div>Dashboard</div>
}
`)
	routes, errs := scanner.ParseTSXFiles(dir, "node:ssr")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	r := routes[0]
	if r.Path != "/dashboard" {
		t.Errorf("wrong path: %s", r.Path)
	}
	if len(r.AuthRoles) != 2 || r.AuthRoles[0] != "user" || r.AuthRoles[1] != "admin" {
		t.Errorf("wrong roles: %v", r.AuthRoles)
	}
}

func TestParseTSXFiles_MissingPath_Error(t *testing.T) {
	dir := t.TempDir()
	writeTSX(t, dir, "Bad.tsx", `// @Page()
export default function Bad() { return null }
`)
	_, errs := scanner.ParseTSXFiles(dir, "node:ssr")
	if len(errs) == 0 {
		t.Fatal("expected annotation error for empty @Page path")
	}
}

func TestParseTSXFiles_MultiFile(t *testing.T) {
	dir := t.TempDir()
	writeTSX(t, dir, "A.tsx", `// @Page(/a)
export default function A() { return null }
`)
	writeTSX(t, dir, "B.tsx", `// @Page(/b)
export default function B() { return null }
`)
	routes, errs := scanner.ParseTSXFiles(dir, "node:ssr")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
}

func TestParseTSXFiles_NonTSXFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	writeTSX(t, dir, "readme.md", `// @Page(/fake)`) // .md, not .tsx
	routes, _ := scanner.ParseTSXFiles(dir, "node:ssr")
	if len(routes) != 0 {
		t.Fatalf("expected no routes from non-tsx file, got %d", len(routes))
	}
}
