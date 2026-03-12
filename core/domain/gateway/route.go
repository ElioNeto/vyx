// Package gateway defines the domain types for the HTTP gateway.
package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"unsafe"
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

// routeNode is a single segment in the trie.
type routeNode struct {
	// static children keyed by the literal segment value.
	children map[string]*routeNode
	// paramChild matches any segment and captures it as a named parameter.
	paramChild *routeNode
	// paramName is the parameter name (without the leading ":").
	paramName string
	// entries holds the matched routes keyed by HTTP method (upper-case).
	entries map[string]RouteEntry
}

func newRouteNode() *routeNode {
	return &routeNode{
		children: make(map[string]*routeNode),
		entries:  make(map[string]RouteEntry),
	}
}

// RouteMap is the in-memory, thread-safe index of all known routes.
// It supports static segments (/api/products) and named path parameters
// (/api/products/:id). Static segments always win over parameter segments
// when both match the same request.
//
// The internal trie is stored under an atomic pointer so that it can be
// hot-swapped by WatchSIGHUP without locking the request path.
type RouteMap struct {
	// root is accessed via atomic load/store (unsafe.Pointer wrapping *routeNode).
	root unsafe.Pointer // *routeNode
}

// NewRouteMap builds a RouteMap from a slice of entries.
func NewRouteMap(entries []RouteEntry) *RouteMap {
	root := buildTrie(entries)
	rm := &RouteMap{}
	atomic.StorePointer(&rm.root, unsafe.Pointer(root))
	return rm
}

// Swap atomically replaces the entire trie with a new one built from entries.
// Safe to call concurrently with Lookup.
func (rm *RouteMap) Swap(entries []RouteEntry) {
	newRoot := buildTrie(entries)
	atomic.StorePointer(&rm.root, unsafe.Pointer(newRoot))
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

// LookupResult holds the matched route entry plus any captured path params.
type LookupResult struct {
	Entry  RouteEntry
	Params map[string]string // e.g. {"id": "123"}
}

// Lookup returns the RouteEntry and captured path params for the given
// method+path, if any. Static segments take priority over param segments.
func (rm *RouteMap) Lookup(method, path string) (LookupResult, bool) {
	root := (*routeNode)(atomic.LoadPointer(&rm.root))
	if root == nil {
		return LookupResult{}, false
	}

	segments := splitPath(path)
	params := make(map[string]string)

	node := traverse(root, segments, params)
	if node == nil {
		return LookupResult{}, false
	}

	entry, ok := node.entries[strings.ToUpper(method)]
	if !ok {
		return LookupResult{}, false
	}

	return LookupResult{Entry: entry, Params: params}, true
}

// splitPath splits a URL path into non-empty segments.
func splitPath(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	result := parts[:0]
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// traverse walks the trie for the given path segments, filling params.
// Returns nil when no match is found.
func traverse(node *routeNode, segments []string, params map[string]string) *routeNode {
	if len(segments) == 0 {
		return node
	}

	seg := segments[0]
	rest := segments[1:]

	// Static children win over param children.
	if child, ok := node.children[seg]; ok {
		if result := traverse(child, rest, params); result != nil {
			return result
		}
	}

	// Fall back to param child.
	if node.paramChild != nil {
		// Use a temporary map so we don't pollute params on backtrack.
		local := copyParams(params)
		local[node.paramChild.paramName] = seg
		if result := traverse(node.paramChild, rest, local); result != nil {
			// Commit the captured params.
			for k, v := range local {
				params[k] = v
			}
			return result
		}
	}

	return nil
}

func copyParams(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// buildTrie constructs a fresh trie root from a slice of RouteEntry.
func buildTrie(entries []RouteEntry) *routeNode {
	root := newRouteNode()
	for _, e := range entries {
		node := root
		for _, seg := range splitPath(e.Path) {
			if strings.HasPrefix(seg, ":") {
				if node.paramChild == nil {
					node.paramChild = newRouteNode()
					node.paramChild.paramName = seg[1:]
				}
				node = node.paramChild
			} else {
				if _, ok := node.children[seg]; !ok {
					node.children[seg] = newRouteNode()
				}
				node = node.children[seg]
			}
		}
		node.entries[strings.ToUpper(e.Method)] = e
	}
	return root
}
