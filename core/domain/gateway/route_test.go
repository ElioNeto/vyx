package gateway_test

import (
	"os"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/gateway"
)

func TestNewRouteMap(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/users", Method: "GET", WorkerID: "w1"},
		{Path: "/api/users/:id", Method: "GET", WorkerID: "w2"},
		{Path: "/api/users", Method: "POST", WorkerID: "w3"},
	}
	rm := gateway.NewRouteMap(entries)
	if rm == nil {
		t.Fatal("expected RouteMap, got nil")
	}
}

func TestLookup_StaticRoute(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/users", Method: "GET", WorkerID: "w1"},
	}
	rm := gateway.NewRouteMap(entries)
	result, ok := rm.Lookup("GET", "/api/users")
	if !ok {
		t.Fatal("expected route found, got false")
	}
	if result.Entry.WorkerID != "w1" {
		t.Errorf("expected worker w1, got %s", result.Entry.WorkerID)
	}
}

func TestLookup_ParamRoute(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/users/:id", Method: "GET", WorkerID: "w2"},
	}
	rm := gateway.NewRouteMap(entries)
	result, ok := rm.Lookup("GET", "/api/users/123")
	if !ok {
		t.Fatal("expected route found, got false")
	}
	if result.Params["id"] != "123" {
		t.Errorf("expected param id=123, got %s", result.Params["id"])
	}
}

func TestLookup_NotFound(t *testing.T) {
	rm := gateway.NewRouteMap(nil)
	_, ok := rm.Lookup("GET", "/nonexistent")
	if ok {
		t.Error("expected route not found, got true")
	}
}

func TestLookup_ParamOverStatic(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/users", Method: "GET", WorkerID: "w1"},
		{Path: "/api/users/:id", Method: "GET", WorkerID: "w2"},
	}
	rm := gateway.NewRouteMap(entries)
	// Static should win
	result, ok := rm.Lookup("GET", "/api/users")
	if !ok {
		t.Fatal("expected static route found")
	}
	if result.Entry.WorkerID != "w1" {
		t.Errorf("expected static route (w1), got %s", result.Entry.WorkerID)
	}
	// Param should also work
	result, ok = rm.Lookup("GET", "/api/users/456")
	if !ok {
		t.Fatal("expected param route found")
	}
	if result.Params["id"] != "456" {
		t.Errorf("expected param id=456, got %s", result.Params["id"])
	}
}

func TestLoadRouteMap(t *testing.T) {
	// Create a temporary route_map.json
	tmpfile := "/tmp/test_route_map.json"
	content := `{"routes": [{"path": "/test", "method": "GET", "worker_id": "w-test"}]}`
	err := os.WriteFile(tmpfile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	defer os.Remove(tmpfile)

	rm, err := gateway.LoadRouteMap(tmpfile)
	if err != nil {
		t.Fatalf("LoadRouteMap failed: %v", err)
	}
	result, ok := rm.Lookup("GET", "/test")
	if !ok {
		t.Fatal("expected route found")
	}
	if result.Entry.WorkerID != "w-test" {
		t.Errorf("expected worker w-test, got %s", result.Entry.WorkerID)
	}
}

func TestSwap(t *testing.T) {
	rm := gateway.NewRouteMap([]gateway.RouteEntry{
		{Path: "/old", Method: "GET", WorkerID: "w-old"},
	})
	// Swap with new entries
	rm.Swap([]gateway.RouteEntry{
		{Path: "/new", Method: "GET", WorkerID: "w-new"},
	})
	// Old should not be found
	_, ok := rm.Lookup("GET", "/old")
	if ok {
		t.Error("expected old route not found after swap")
	}
	// New should be found
	result, ok := rm.Lookup("GET", "/new")
	if !ok {
		t.Fatal("expected new route found after swap")
	}
	if result.Entry.WorkerID != "w-new" {
		t.Errorf("expected worker w-new, got %s", result.Entry.WorkerID)
	}
}
