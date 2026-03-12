package heartbeat

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// WorkerLister is the subset of the worker repository used by the Sender
// to discover which workers are currently alive.
type WorkerLister interface {
	LiveWorkerIDs(ctx context.Context) ([]string, error)
}

// Sender periodically sends a TypeHeartbeat frame from the core to every
// connected worker over the IPC transport. This lets workers detect core
// unavailability independently of the worker→core heartbeat loop.
//
// The Sender is the application-layer owner of the core→worker heartbeat;
// the OS process manager (infrastructure/process) is intentionally kept
// free of transport concerns.
type Sender struct {
	transport ipc.Transport
	lister    WorkerLister
	cfg       Config
	log       *zap.Logger
}

// NewSender creates a Sender that writes heartbeat frames to all live workers.
func NewSender(
	transport ipc.Transport,
	lister WorkerLister,
	cfg Config,
	log *zap.Logger,
) *Sender {
	return &Sender{
		transport: transport,
		lister:    lister,
		cfg:       cfg,
		log:       log,
	}
}

// Run starts the heartbeat send loop. It ticks every cfg.Interval, fetches
// the list of live worker IDs, and sends a TypeHeartbeat frame to each one.
// Blocks until ctx is cancelled.
func (s *Sender) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sendAll(ctx)
		}
	}
}

// sendAll sends a TypeHeartbeat frame to every live worker.
// A failure for one worker is logged and skipped — it must not block others.
func (s *Sender) sendAll(ctx context.Context) {
	ids, err := s.lister.LiveWorkerIDs(ctx)
	if err != nil {
		s.log.Error("heartbeat sender: failed to list workers", zap.Error(err))
		return
	}

	msg := ipc.Message{Type: ipc.TypeHeartbeat, Payload: []byte{}}

	for _, id := range ids {
		if err := s.transport.Send(ctx, id, msg); err != nil {
			s.log.Warn("heartbeat sender: failed to send heartbeat to worker",
				zap.String("worker_id", id),
				zap.Error(err),
			)
			continue
		}
		s.log.Debug("heartbeat sender: sent heartbeat",
			zap.String("worker_id", id),
		)
	}
}
