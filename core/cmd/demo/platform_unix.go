//go:build !windows

package main

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/infrastructure/ipc/uds"
)

func transportName() string { return "Unix Domain Socket" }

func dialPlatform(ctx context.Context, workerID string) (*uds.Client, error) {
	socketPath := fmt.Sprintf("%s/%s.sock", uds.DefaultSocketDir, workerID)
	return uds.Dial(ctx, socketPath)
}
