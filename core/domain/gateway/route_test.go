package gateway_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

func TestRouteMap_Lookup_Found(t *testing.T) {
	entries := []dgw.RouteEntry{
		{Method: "GET", Path: "/api/users", WorkerID: "go:api"},
		{Method: "POST", Path: "/api/users", WorkerID: "go:api"},
	}
	rm := dgw.NewRouteMap(entries)

	e, ok := rm.Lookup("GET", "/api/users")
	if !ok {
		t.Fatal("expected route to be found")
	}
	if e.WorkerID != "go:api" {
		t.Errorf("unexpected worker_id: %q", e.WorkerID)
	}
}

func TestRouteMap_Lookup_NotFound(t *testing.T) {
	rm := dgw.NewRouteMap(nil)
	_, ok := rm.Lookup("DELETE", "/nonexistent")
	if ok {
		t.Error("expected route not to be found")
	}
}

func TestLoadRouteMap_ValidFile(t *testing.T) {
	payload := map[string]any{
		"routes": []map[string]any{
			{"method": "GET", "path": "/ping", "worker_id": "go:api", "auth_roles": []string{}, "validate": "", "type": "api"},
		},
	}
	data, _ := json.Marshal(payload)

	dir := t.TempDir()
	path := filepath.Join(dir, "route_map.json")
	_ = os.WriteFile(path, data, 0644)

	rm, err := dgw.LoadRouteMap(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, ok := rm.Lookup("GET", "/ping")
	if !ok {
		t.Error("expected /ping to be found after loading route_map.json")
	}
}

func TestLoadRouteMap_MissingFile(t *testing.T) {
	_, err := dgw.LoadRouteMap("/nonexistent/route_map.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
