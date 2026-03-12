//go:build windows

package main

import (
	"context"

	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
)

func transportName() string { return "Named Pipe" }

func dialPlatform(ctx context.Context, workerID string) (*uds.Client, error) {
	return uds.DialNamedPipe(ctx, workerID)
}
