package heartbeat

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// Receiver starts one heartbeat.Loop per live worker and keeps the set of
// running loops in sync with the worker repository.
type Receiver struct {
	transport ipc.Transport
	lister    WorkerLister
	service   LifecycleService
	cfg       Config
	log       *zap.Logger

	mu      sync.Mutex
	running map[string]context.CancelFunc // workerID → loop cancel
}

// NewReceiver creates a Receiver. service may be nil at construction time
// and set later via SetService (used to break circular dependency in main).
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

// SetService wires the lifecycle service after construction.
// Call this before Run/StartLoop to break the circular dependency:
//
//	hbReceiver := heartbeat.NewReceiver(transport, repo, nil, cfg, log)
//	service    := lifecycle.NewService(repo, manager, pub, transport, hbReceiver)
//	hbReceiver.SetService(service)
func (r *Receiver) SetService(svc LifecycleService) {
	r.mu.Lock()
	r.service = svc
	r.mu.Unlock()
}

// Run reconciles the set of running heartbeat loops with the live worker list
// on every tick. Blocks until ctx is cancelled.
func (r *Receiver) Run(ctx context.Context) {
	r.reconcile(ctx)

	ticker := newTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.stopAll()
			return
		case <-ticker.Chan():
			r.reconcile(ctx)
		}
	}
}

// StartLoop starts a heartbeat read loop for workerID if one is not already
// running. Called by main after the initial spawn.
func (r *Receiver) StartLoop(ctx context.Context, workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.running[workerID]; ok {
		return // loop already running
	}
	r.launchLocked(ctx, workerID)
}

// RestartLoop cancels any existing loop for workerID and immediately starts a
// fresh one. Used after a worker restart so the new Named Pipe / UDS
// connection is picked up instead of the stale handle from the previous run.
func (r *Receiver) RestartLoop(ctx context.Context, workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cancel, ok := r.running[workerID]; ok {
		cancel()
		delete(r.running, workerID)
		r.log.Info("heartbeat receiver: cancelled old loop before restart",
			zap.String("worker_id", workerID),
		)
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

	for id := range live {
		if _, ok := r.running[id]; !ok {
			r.launchLocked(ctx, id)
		}
	}

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
