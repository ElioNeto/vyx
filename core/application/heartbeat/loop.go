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
	"strings"
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
	MarkRunning(ctx context.Context, workerID string) error
}

// Config holds tuning parameters for the heartbeat loop.
type Config struct {
	// Interval is how often the sender pings workers.
	Interval time.Duration
	// ReadTimeout is the deadline the loop gives each Receive call.
	// It must be larger than Interval so the full round-trip
	// (sender sends ping → worker echoes back) fits inside the window.
	// When zero, defaults to Interval * 2.
	ReadTimeout time.Duration
	// MissedThreshold is the number of consecutive missed heartbeats before
	// a worker is marked unhealthy. Default: 2 (i.e., 10s with 5s interval).
	MissedThreshold int
	// ConnectGrace is the window after loop start during which
	// "worker not connected" errors are tolerated without incrementing missed.
	// This gives the worker process time to dial the IPC socket.
	ConnectGrace time.Duration
	// RetryInterval is the sleep between retries during the grace period.
	RetryInterval time.Duration
}

// DefaultConfig returns production-safe defaults.
// ConnectGrace of 30s gives the worker enough time to spawn and connect;
// RetryInterval of 500ms keeps the polling lightweight.
func DefaultConfig() Config {
	return Config{
		Interval:        5 * time.Second,
		ReadTimeout:     10 * time.Second, // 2× Interval — room for sender→worker round-trip
		MissedThreshold: 2,
		ConnectGrace:    30 * time.Second,
		RetryInterval:   500 * time.Millisecond,
	}
}

// isNotConnectedError returns true when err signals that the worker has not
// yet established its IPC connection (as opposed to a real missed heartbeat).
func isNotConnectedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ipc: worker is not connected")
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
func (l *Loop) Run(ctx context.Context) {
	missed := 0
	startTime := time.Now()

	for {
		if ctx.Err() != nil {
			return
		}

		msg, err := l.receiveWithTimeout(ctx, startTime)
		if err != nil {
			missed = l.handleReceiveError(ctx, missed, startTime, err)
			if missed < 0 {
				return
			}
			continue
		}

		missed = 0
		l.handleMessage(ctx, msg)
	}
}

func (l *Loop) receiveWithTimeout(ctx context.Context, startTime time.Time) (ipc.Message, error) {
	readTimeout := l.cfg.ReadTimeout
	if readTimeout == 0 {
		readTimeout = l.cfg.Interval * 2
	}
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return l.transport.Receive(readCtx, l.workerID)
}

func (l *Loop) handleReceiveError(ctx context.Context, missed int, startTime time.Time, err error) int {
	if ctx.Err() != nil {
		return -1
	}

	if isNotConnectedError(err) && time.Since(startTime) < l.cfg.ConnectGrace {
		l.log.Debug("worker not yet connected, waiting for IPC handshake",
			zap.String("worker_id", l.workerID),
			zap.Duration("elapsed", time.Since(startTime)),
			zap.Duration("grace", l.cfg.ConnectGrace),
		)
		select {
		case <-ctx.Done():
			return -1
		case <-time.After(l.cfg.RetryInterval):
		}
		return missed
	}

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
		return -1
	}
	return missed
}

func (l *Loop) handleMessage(ctx context.Context, msg ipc.Message) {
	switch msg.Type {
	case ipc.TypeHandshake:
		l.log.Info("handshake received on heartbeat loop — worker connected",
			zap.String("worker_id", l.workerID),
		)
		_ = l.service.MarkRunning(ctx, l.workerID)

	case ipc.TypeHeartbeat:
		if err := l.service.RecordHeartbeat(ctx, l.workerID); err != nil {
			if err == worker.ErrNotFound {
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
