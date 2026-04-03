//go:build windows

package main

import (
	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
)

// isWindows returns true on Windows builds.
func isWindows() bool { return true }

// platformTransport returns the Named Pipe transport on Windows.
func platformTransport(_ string) ipc.Transport {
	return uds.NewNamedPipeTransport()
}
