//go:build !windows

package main

import (
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
)

// isWindows returns false on non-Windows builds.
func isWindows() bool { return false }

// platformTransport returns the UDS-backed transport on Unix systems.
func platformTransport(socketDir string) ipc.Transport {
	return uds.New(socketDir)
}
