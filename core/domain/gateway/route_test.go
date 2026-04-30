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
	rm.Swap([]gateway.RouteEntry{
		{Path: "/new", Method: "GET", WorkerID: "w-new"},
	})
	_, ok := rm.Lookup("GET", "/old")
	if ok {
		t.Error("expected old route not found after swap")
	}
	result, ok := rm.Lookup("GET", "/new")
	if !ok {
		t.Fatal("expected new route found after swap")
	}
	if result.Entry.WorkerID != "w-new" {
		t.Errorf("expected worker w-new, got %s", result.Entry.WorkerID)
	}
}

func TestLoadRouteMap_InvalidJSON(t *testing.T) {
	tmpfile := "/tmp/test_invalid_route_map.json"
	content := `invalid json`
	err := os.WriteFile(tmpfile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	defer os.Remove(tmpfile)

	rm, err := gateway.LoadRouteMap(tmpfile)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if rm != nil {
		t.Error("expected nil RouteMap for invalid JSON")
	}
}

func TestLoadRouteMap_MissingRoutesField(t *testing.T) {
	tmpfile := "/tmp/test_missing_routes.json"
	content := `{"version": "1.0"}`
	err := os.WriteFile(tmpfile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	defer os.Remove(tmpfile)

	rm, err := gateway.LoadRouteMap(tmpfile)
	if err != nil {
		t.Fatalf("LoadRouteMap failed: %v", err)
	}
	if rm == nil {
		t.Fatal("expected RouteMap for missing routes field")
	}
	_, ok := rm.Lookup("GET", "/test")
	if ok {
		t.Error("expected no route found")
	}
}

func TestLookup_EmptyRouteMap(t *testing.T) {
	rm := gateway.NewRouteMap(nil)
	_, ok := rm.Lookup("GET", "/test")
	if ok {
		t.Error("expected no route found in empty map")
	}
}

func TestLookup_MethodNotFound(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/test", Method: "GET", WorkerID: "w1"},
	}
	rm := gateway.NewRouteMap(entries)
	_, ok := rm.Lookup("POST", "/api/test")
	if ok {
		t.Error("expected no route found for unregistered method")
	}
}

func TestLoadRouteMap_ReadError(t *testing.T) {
	rm, err := gateway.LoadRouteMap("/tmp/non-existent-file-12345.json")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if rm != nil {
		t.Error("expected nil RouteMap for read error")
	}
}

func TestLookup_NilRoot(t *testing.T) {
	rm := gateway.NewRouteMap(nil)
	_, ok := rm.Lookup("GET", "/test")
	if ok {
		t.Error("expected false for nil root")
	}
}

func TestLookup_WithEmptyMethod(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/test", Method: "GET", WorkerID: "w1"},
	}
	rm := gateway.NewRouteMap(entries)
	_, ok := rm.Lookup("", "/api/test")
	if ok {
		t.Error("expected no route found for empty method")
	}
}

func TestLookup_WithExtraSegments(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/test", Method: "GET", WorkerID: "w1"},
	}
	rm := gateway.NewRouteMap(entries)
	// Extra segments should not match
	_, ok := rm.Lookup("GET", "/api/test/extra")
	if ok {
		t.Error("expected no route found for extra segments")
	}
}

func TestLookup_ParamRoute_ExtraSegments(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/:resource", Method: "GET", WorkerID: "w1"},
	}
	rm := gateway.NewRouteMap(entries)
	// Extra segments should not match
	_, ok := rm.Lookup("GET", "/api/users/extra")
	if ok {
		t.Error("expected no route found for extra segments after param")
	}
}

func TestLookup_RootPath(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/", Method: "GET", WorkerID: "w-root"},
	}
	rm := gateway.NewRouteMap(entries)
	result, ok := rm.Lookup("GET", "/")
	if !ok {
		t.Fatal("expected root route found")
	}
	if result.Entry.WorkerID != "w-root" {
		t.Errorf("expected w-root, got %s", result.Entry.WorkerID)
	}
}

func TestLookup_TrailingSlash(t *testing.T) {
	entries := []gateway.RouteEntry{
		{Path: "/api/test", Method: "GET", WorkerID: "w1"},
	}
	rm := gateway.NewRouteMap(entries)
	// Trailing slash should match (splitPath trims it)
	result, ok := rm.Lookup("GET", "/api/test/")
	if !ok {
		t.Fatal("expected route found with trailing slash")
	}
	if result.Entry.WorkerID != "w1" {
		t.Errorf("expected w1, got %s", result.Entry.WorkerID)
	}
}

func TestTraverse_EmptyParams(t *testing.T) {
	// This should trigger copyParams with empty map
	entries := []gateway.RouteEntry{
		{Path: "/api/:id", Method: "GET", WorkerID: "w1"},
	}
	rm := gateway.NewRouteMap(entries)
	
	// Lookup with a value should work
	result, ok := rm.Lookup("GET", "/api/123")
	if !ok {
		t.Fatal("expected route found")
	}
	if result.Params["id"] != "123" {
		t.Errorf("expected id=123, got %s", result.Params["id"])
	}
}

func TestTraverse_WithExistingParams(t *testing.T) {
	// This should trigger copyParams with non-empty map
	entries := []gateway.RouteEntry{
		{Path: "/api/:resource/:id", Method: "GET", WorkerID: "w1"},
	}
	rm := gateway.NewRouteMap(entries)
	
	result, ok := rm.Lookup("GET", "/api/users/123")
	if !ok {
		t.Fatal("expected route found")
	}
	if result.Params["resource"] != "users" {
		t.Errorf("expected resource=users, got %s", result.Params["resource"])
	}
	if result.Params["id"] != "123" {
		t.Errorf("expected id=123, got %s", result.Params["id"])
	}
}
