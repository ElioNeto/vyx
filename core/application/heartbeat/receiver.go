package heartbeat

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// Receiver starts one heartbeat.Loop per live worker and keeps the set of
// running loops in sync with the worker repository.
//
// Design: Receiver is intentionally separate from Sender so that the two
// traffic directions (core→worker and worker→core) can be reasoned about
// independently. Receiver satisfies spec §5.4: "Workers send periodic
// heartbeats every 5s to the core."
type Receiver struct {
	transport ipc.Transport
	lister    WorkerLister
	service   LifecycleService
	cfg       Config
	log       *zap.Logger

	mu      sync.Mutex
	running map[string]context.CancelFunc // workerID → loop cancel
}

// NewReceiver creates a Receiver wired with all required dependencies.
func NewReceiver(
	transport ipc.Transport,
	lister WorkerLister,
	service LifecycleService,
	cfg Config,
	log *zap.Logger,
) *Receiver {
	return &Receiver{
		transport: transport,
		lister:    lister,
		service:   service,
		cfg:       cfg,
		log:       log,
		running:   make(map[string]context.CancelFunc),
	}
}

// Run reconciles the set of running heartbeat loops with the live worker list
// on every tick. New workers get a loop; workers that disappeared have their
// loop cancelled. Blocks until ctx is cancelled.
func (r *Receiver) Run(ctx context.Context) {
	// Kick off loops for workers that are already alive at startup.
	r.reconcile(ctx)

	ticker := newTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.stopAll()
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

// StartLoop manually starts a heartbeat read loop for workerID. This is the
// hook called by main.go right after a worker is spawned so we do not have to
// wait for the next reconcile tick.
func (r *Receiver) StartLoop(ctx context.Context, workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.running[workerID]; ok {
		return // loop already running
	}
	r.launchLocked(ctx, workerID)
}

// reconcile starts loops for workers that do not have one yet and cancels
// loops for workers that are no longer alive.
func (r *Receiver) reconcile(ctx context.Context) {
	ids, err := r.lister.LiveWorkerIDs(ctx)
	if err != nil {
		r.log.Error("heartbeat receiver: failed to list workers", zap.Error(err))
		return
	}

	live := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		live[id] = struct{}{}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Start loops for newly appeared workers.
	for id := range live {
		if _, ok := r.running[id]; !ok {
			r.launchLocked(ctx, id)
		}
	}

	// Cancel loops for workers that are no longer in the live set.
	for id, cancel := range r.running {
		if _, ok := live[id]; !ok {
			cancel()
			delete(r.running, id)
			r.log.Info("heartbeat receiver: stopped loop for departed worker",
				zap.String("worker_id", id),
			)
		}
	}
}

// launchLocked starts a Loop goroutine for workerID. Caller must hold r.mu.
func (r *Receiver) launchLocked(ctx context.Context, workerID string) {
	loopCtx, cancel := context.WithCancel(ctx)
	r.running[workerID] = cancel

	loop := New(workerID, r.transport, r.service, r.cfg, r.log)
	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.running, workerID)
			r.mu.Unlock()
		}()
		loop.Run(loopCtx)
	}()

	r.log.Info("heartbeat receiver: started loop",
		zap.String("worker_id", workerID),
	)
}

// stopAll cancels every running loop. Called when the root context is done.
func (r *Receiver) stopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, cancel := range r.running {
		cancel()
		delete(r.running, id)
		r.log.Info("heartbeat receiver: stopped loop on shutdown",
			zap.String("worker_id", id),
		)
	}
}
