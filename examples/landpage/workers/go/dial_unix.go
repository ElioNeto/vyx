//go:build !windows

package main

import (
	"fmt"
	"net"
)

// dialNamedPipe is a stub on non-Windows platforms.
// The real connection path goes through net.Dial("unix", ...) in main.go.
func dialNamedPipe(path string) (net.Conn, error) {
	return nil, fmt.Errorf("dialNamedPipe called on non-Windows platform (path: %s)", path)
}
