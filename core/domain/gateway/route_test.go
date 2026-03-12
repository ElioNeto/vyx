package gateway

import (
	"testing"
)

func TestRouteMap_StaticLookup(t *testing.T) {
	rm := NewRouteMap([]RouteEntry{
		{Path: "/api/products", Method: "GET", WorkerID: "node:api"},
	})
	res, ok := rm.Lookup("GET", "/api/products")
	if !ok {
		t.Fatal("expected match for static route")
	}
	if res.Entry.WorkerID != "node:api" {
		t.Errorf("unexpected workerID: %s", res.Entry.WorkerID)
	}
}

func TestRouteMap_PathParam(t *testing.T) {
	rm := NewRouteMap([]RouteEntry{
		{Path: "/api/products/:id", Method: "GET", WorkerID: "node:api"},
	})
	res, ok := rm.Lookup("GET", "/api/products/123")
	if !ok {
		t.Fatal("expected match for param route")
	}
	if res.Params["id"] != "123" {
		t.Errorf("expected param id=123, got %v", res.Params)
	}
}

func TestRouteMap_StaticWinsOverParam(t *testing.T) {
	rm := NewRouteMap([]RouteEntry{
		{Path: "/api/products/featured", Method: "GET", WorkerID: "node:featured"},
		{Path: "/api/products/:id", Method: "GET", WorkerID: "node:api"},
	})
	res, ok := rm.Lookup("GET", "/api/products/featured")
	if !ok {
		t.Fatal("expected match")
	}
	if res.Entry.WorkerID != "node:featured" {
		t.Errorf("static should win, got workerID=%s", res.Entry.WorkerID)
	}
}

func TestRouteMap_MultipleParams(t *testing.T) {
	rm := NewRouteMap([]RouteEntry{
		{Path: "/api/orders/:orderId/items/:itemId", Method: "GET", WorkerID: "node:orders"},
	})
	res, ok := rm.Lookup("GET", "/api/orders/42/items/7")
	if !ok {
		t.Fatal("expected match")
	}
	if res.Params["orderId"] != "42" || res.Params["itemId"] != "7" {
		t.Errorf("unexpected params: %v", res.Params)
	}
}

func TestRouteMap_NoMatch(t *testing.T) {
	rm := NewRouteMap([]RouteEntry{
		{Path: "/api/products", Method: "GET", WorkerID: "node:api"},
	})
	_, ok := rm.Lookup("GET", "/api/orders")
	if ok {
		t.Fatal("expected no match")
	}
}

func TestRouteMap_MethodMismatch(t *testing.T) {
	rm := NewRouteMap([]RouteEntry{
		{Path: "/api/products", Method: "GET", WorkerID: "node:api"},
	})
	_, ok := rm.Lookup("POST", "/api/products")
	if ok {
		t.Fatal("expected no match for wrong method")
	}
}

func TestRouteMap_Swap(t *testing.T) {
	rm := NewRouteMap([]RouteEntry{
		{Path: "/old", Method: "GET", WorkerID: "node:old"},
	})
	_, ok := rm.Lookup("GET", "/old")
	if !ok {
		t.Fatal("expected old route to match before swap")
	}

	rm.Swap([]RouteEntry{
		{Path: "/new", Method: "GET", WorkerID: "node:new"},
	})

	_, ok = rm.Lookup("GET", "/old")
	if ok {
		t.Fatal("old route should not match after swap")
	}
	res, ok := rm.Lookup("GET", "/new")
	if !ok {
		t.Fatal("new route should match after swap")
	}
	if res.Entry.WorkerID != "node:new" {
		t.Errorf("unexpected workerID after swap: %s", res.Entry.WorkerID)
	}
}
