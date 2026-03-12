//go:build !windows

package uds

import "github.com/ElioNeto/vyx/core/domain/ipc"

// PlatformTransport returns the UDS-backed Transport on Unix systems.
// On Windows, named_pipe_windows.go provides the equivalent implementation.
func PlatformTransport() ipc.Transport {
	return New(DefaultSocketDir)
}

// Ensure Transport satisfies the domain interface at compile time.
var _ ipc.Transport = (*Transport)(nil)
