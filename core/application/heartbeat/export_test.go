package heartbeat

import (
	"context"
	"time"

	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// ExportIsNotConnectedError exposes the unexported isNotConnectedError for testing.
func ExportIsNotConnectedError(err error) bool {
	return isNotConnectedError(err)
}

// ExportHandleReceiveError exposes the unexported handleReceiveError logic for testing.
// It returns the new missed count, or -1 if the loop should exit.
func (l *Loop) ExportHandleReceiveError(ctx context.Context, missed int, startTime time.Time, err error) int {
	return l.handleReceiveError(ctx, missed, startTime, err)
}

// ExportHandleMessage exposes the unexported handleMessage for testing.
func (l *Loop) ExportHandleMessage(ctx context.Context, msg ipc.Message) {
	l.handleMessage(ctx, msg)
}
