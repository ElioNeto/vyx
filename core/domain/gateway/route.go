// Package gateway defines the domain types for the HTTP gateway.
package gateway

import (
	"encoding/json"
	"fmt"
	"os"
)

// RouteEntry represents a single entry from route_map.json.
type RouteEntry struct {
	Path      string   `json:"path"`
	Method    string   `json:"method"`
	WorkerID  string   `json:"worker_id"`
	AuthRoles []string `json:"auth_roles"`
	Validate  string   `json:"validate"`
	Type      string   `json:"type"`
}

// RouteMap is the in-memory index of all known routes.
type RouteMap struct {
	entries map[string]RouteEntry // key: "METHOD /path"
}

// NewRouteMap builds a RouteMap from a slice of entries.
func NewRouteMap(entries []RouteEntry) *RouteMap {
	m := &RouteMap{entries: make(map[string]RouteEntry, len(entries))}
	for _, e := range entries {
		m.entries[e.Method+" "+e.Path] = e
	}
	return m
}

// LoadRouteMap reads and parses route_map.json from the given path.
func LoadRouteMap(path string) (*RouteMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("gateway: read route map %s: %w", path, err)
	}
	var payload struct {
		Routes []RouteEntry `json:"routes"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("gateway: parse route map: %w", err)
	}
	return NewRouteMap(payload.Routes), nil
}

// Lookup returns the RouteEntry for the given method+path pair, if any.
func (r *RouteMap) Lookup(method, path string) (RouteEntry, bool) {
	e, ok := r.entries[method+" "+path]
	return e, ok
}
