// Package heartbeat implements the 5-second heartbeat read loop.
//
// The loop runs per worker: it continuously reads messages from the transport,
// dispatches TypeHeartbeat frames to lifecycle.Service.RecordHeartbeat, and
// forwards TypeResponse / TypeError frames to the pending request dispatcher
// (wired in issue #4).
//
// Architecture note: this is an application-layer use case that coordinates
// two domain ports (ipc.Transport and the lifecycle service). It has no
// knowledge of sockets, framing, or OS specifics.
package heartbeat

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/domain/worker"
)

// LifecycleService is the subset of lifecycle.Service used by the heartbeat loop.
// Defined as a local interface to keep the dependency minimal and testable.
type LifecycleService interface {
	RecordHeartbeat(ctx context.Context, workerID string) error
	MarkUnhealthy(ctx context.Context, workerID string) error
}

// Config holds tuning parameters for the heartbeat loop.
type Config struct {
	// Interval is how often the loop checks for a missed heartbeat.
	// Workers are expected to send a heartbeat at least once per Interval.
	Interval time.Duration
	// MissedThreshold is the number of consecutive missed heartbeats before
	// a worker is marked unhealthy. Default: 2 (i.e., 10s with 5s interval).
	MissedThreshold int
}

// DefaultConfig returns production-safe defaults (5s interval, 2 missed = unhealthy).
func DefaultConfig() Config {
	return Config{
		Interval:        5 * time.Second,
		MissedThreshold: 2,
	}
}

// Loop reads messages from a single worker's transport connection and handles
// heartbeat and error frames. It runs for the lifetime of a worker connection.
type Loop struct {
	workerID  string
	transport ipc.Transport
	service   LifecycleService
	cfg       Config
	log       *zap.Logger
}

// New creates a heartbeat Loop for the given worker.
func New(
	workerID string,
	transport ipc.Transport,
	service LifecycleService,
	cfg Config,
	log *zap.Logger,
) *Loop {
	return &Loop{
		workerID:  workerID,
		transport: transport,
		service:   service,
		cfg:       cfg,
		log:       log,
	}
}

// Run starts the heartbeat read loop. It blocks until ctx is cancelled or the
// transport returns an unrecoverable error (e.g., worker disconnected).
//
// Design: instead of a ticker, we use a deadline-per-read approach:
//   - Set a read deadline of (now + interval) on each iteration.
//   - If the read times out → the worker missed its heartbeat window.
//   - If we receive TypeHeartbeat → call RecordHeartbeat and reset.
//   - After MissedThreshold timeouts → call MarkUnhealthy.
//
// This avoids the "thundering herd" problem of a global ticker firing for all
// workers simultaneously and gives each worker an independent deadline.
func (l *Loop) Run(ctx context.Context) {
	missed := 0

	for {
		if ctx.Err() != nil {
			return
		}

		// Create a child context with the heartbeat read deadline.
		readCtx, cancel := context.WithTimeout(ctx, l.cfg.Interval)
		msg, err := l.transport.Receive(readCtx, l.workerID)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				// Parent cancelled — clean exit.
				return
			}

			// Treat any receive error as a missed heartbeat.
			missed++
			l.log.Warn("worker heartbeat missed",
				zap.String("worker_id", l.workerID),
				zap.Int("missed", missed),
				zap.Error(err),
			)

			if missed >= l.cfg.MissedThreshold {
				l.log.Error("worker exceeded missed heartbeat threshold, marking unhealthy",
					zap.String("worker_id", l.workerID),
					zap.Int("threshold", l.cfg.MissedThreshold),
				)
				_ = l.service.MarkUnhealthy(ctx, l.workerID)
				return
			}
			continue
		}

		switch msg.Type {
		case ipc.TypeHeartbeat:
			missed = 0
			if err := l.service.RecordHeartbeat(ctx, l.workerID); err != nil {
				if err == worker.ErrNotFound {
					// Worker was deregistered; stop the loop.
					return
				}
				l.log.Error("RecordHeartbeat failed",
					zap.String("worker_id", l.workerID),
					zap.Error(err),
				)
			}
			l.log.Debug("heartbeat received",
				zap.String("worker_id", l.workerID),
			)

		case ipc.TypeResponse, ipc.TypeError:
			// Responses are handled by the request dispatcher (issue #4).
			// Log unexpected frames here so they are not silently dropped.
			l.log.Debug("non-heartbeat frame received on heartbeat loop",
				zap.String("worker_id", l.workerID),
				zap.String("type", msg.Type.String()),
			)

		default:
			l.log.Warn("unexpected message type on heartbeat loop",
				zap.String("worker_id", l.workerID),
				zap.String("type", msg.Type.String()),
			)
		}
	}
}
