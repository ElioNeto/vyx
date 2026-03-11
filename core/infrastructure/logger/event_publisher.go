// Package logger provides a zap-based implementation of worker.EventPublisher
// that emits structured JSON log entries for every domain event.
package logger

import (
	"context"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

// EventPublisher writes worker lifecycle events as structured JSON using zap.
type EventPublisher struct {
	log *zap.Logger
}

// New creates an EventPublisher wrapping the given zap logger.
func New(log *zap.Logger) *EventPublisher {
	return &EventPublisher{log: log}
}

// Publish emits a structured log entry for the given domain event.
func (p *EventPublisher) Publish(_ context.Context, event worker.Event) {
	p.log.Info("worker event",
		zap.String("event", string(event.Type)),
		zap.String("worker_id", event.WorkerID),
		zap.String("state", string(event.State)),
		zap.Time("timestamp", event.Timestamp),
		zap.String("details", event.Details),
	)
}
