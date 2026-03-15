// Package lifecycle contains the application use cases for worker lifecycle management.
// This layer orchestrates domain entities using the interfaces defined in the domain.
package lifecycle

import (
	"context"
	"time"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	"github.com/ElioNeto/vyx/core/domain/worker"
)

// ReceiverStarter is the subset of heartbeat.Receiver used by the lifecycle
// service to re-arm the read loop after a worker restart.
type ReceiverStarter interface {
	StartLoop(ctx context.Context, workerID string)
}

// Service implements all use cases related to worker lifecycle.
type Service struct {
	repo      worker.Repository
	manager   worker.Manager
	publisher worker.EventPublisher
	transport ipc.Transport
	receiver  ReceiverStarter
}

// NewService constructs a lifecycle Service with all required dependencies injected.
func NewService(
	repo worker.Repository,
	manager worker.Manager,
	publisher worker.EventPublisher,
	transport ipc.Transport,
	receiver ReceiverStarter,
) *Service {
	return &Service{
		repo:      repo,
		manager:   manager,
		publisher: publisher,
		transport: transport,
		receiver:  receiver,
	}
}

// SpawnWorker registers a new worker and starts its process.
func (s *Service) SpawnWorker(ctx context.Context, id, command string, args []string, workDir string) (*worker.Worker, error) {
	if command == "" {
		return nil, worker.ErrInvalidCommand
	}

	w := &worker.Worker{
		ID:        id,
		Command:   command,
		Args:      args,
		WorkDir:   workDir,
		State:     worker.StateStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Save(ctx, w); err != nil {
		return nil, err
	}

	if err := s.manager.Spawn(ctx, w); err != nil {
		w.State = worker.StateStopped
		w.UpdatedAt = time.Now()
		_ = s.repo.Save(ctx, w)
		s.publish(ctx, worker.EventSpawned, w, err.Error())
		return nil, worker.ErrSpawnFailed
	}

	w.State = worker.StateRunning
	w.UpdatedAt = time.Now()
	_ = s.repo.Save(ctx, w)
	s.publish(ctx, worker.EventRunning, w, "")

	return w, nil
}

// StopWorker gracefully stops a running worker.
func (s *Service) StopWorker(ctx context.Context, id string) error {
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return worker.ErrNotFound
	}

	if err := s.manager.Stop(ctx, id); err != nil {
		return err
	}

	w.State = worker.StateStopped
	w.UpdatedAt = time.Now()
	_ = s.repo.Save(ctx, w)
	s.publish(ctx, worker.EventStopped, w, "graceful stop")

	return nil
}

// StopAll gracefully stops all running workers. Used on SIGTERM.
func (s *Service) StopAll(ctx context.Context) error {
	workers, err := s.repo.FindAll(ctx)
	if err != nil {
		return err
	}

	var lastErr error
	for _, w := range workers {
		if !w.IsAlive() {
			continue
		}
		if err := s.StopWorker(ctx, w.ID); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// RecordHeartbeat updates the last heartbeat timestamp for a worker.
func (s *Service) RecordHeartbeat(ctx context.Context, id string) error {
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return worker.ErrNotFound
	}

	w.LastHeartbeat = time.Now()
	w.UpdatedAt = time.Now()
	if w.State == worker.StateUnhealthy {
		w.State = worker.StateRunning
	}

	s.publish(ctx, worker.EventHeartbeat, w, "")
	return s.repo.Save(ctx, w)
}

// MarkUnhealthy transitions a worker to the unhealthy state (missed heartbeat).
func (s *Service) MarkUnhealthy(ctx context.Context, id string) error {
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return worker.ErrNotFound
	}

	w.State = worker.StateUnhealthy
	w.UpdatedAt = time.Now()
	s.publish(ctx, worker.EventUnhealthy, w, "missed heartbeat")
	return s.repo.Save(ctx, w)
}

// MarkRunning transitions a worker to StateRunning after a successful handshake.
func (s *Service) MarkRunning(ctx context.Context, id string) error {
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return worker.ErrNotFound
	}

	if w.State == worker.StateRunning {
		return nil
	}

	w.State = worker.StateRunning
	w.UpdatedAt = time.Now()
	s.publish(ctx, worker.EventRunning, w, "handshake complete")
	return s.repo.Save(ctx, w)
}

// RestartWorker stops and re-spawns a worker (called by the monitor after backoff).
// It also recreates the IPC transport endpoint so the worker process can
// reconnect after restart (Named Pipe on Windows, UDS on Unix).
func (s *Service) RestartWorker(ctx context.Context, id string) error {
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return worker.ErrNotFound
	}

	w.State = worker.StateRestarting
	w.RestartCount++
	w.UpdatedAt = time.Now()
	_ = s.repo.Save(ctx, w)
	s.publish(ctx, worker.EventRestarting, w, "automatic restart")

	_ = s.manager.Stop(ctx, id)

	// Recreate the IPC endpoint so the restarted worker process can connect.
	// Without this, the Named Pipe handle (Windows) or UDS file (Unix) is
	// stale and the acceptPipe goroutine is no longer listening.
	if s.transport != nil {
		_ = s.transport.Deregister(ctx, id)
		if err := s.transport.Register(ctx, id); err != nil {
			w.State = worker.StateStopped
			w.UpdatedAt = time.Now()
			_ = s.repo.Save(ctx, w)
			s.publish(ctx, worker.EventStopped, w, "restart failed: transport re-register: "+err.Error())
			return worker.ErrSpawnFailed
		}
	}

	if err := s.manager.Spawn(ctx, w); err != nil {
		w.State = worker.StateStopped
		w.UpdatedAt = time.Now()
		_ = s.repo.Save(ctx, w)
		s.publish(ctx, worker.EventStopped, w, "restart failed: "+err.Error())
		return worker.ErrSpawnFailed
	}

	w.State = worker.StateRunning
	w.UpdatedAt = time.Now()
	_ = s.repo.Save(ctx, w)
	s.publish(ctx, worker.EventRunning, w, "restarted successfully")

	// Re-arm the heartbeat read loop for the new connection.
	if s.receiver != nil {
		s.receiver.StartLoop(ctx, id)
	}

	return nil
}

func (s *Service) publish(ctx context.Context, eventType worker.EventType, w *worker.Worker, details string) {
	s.publisher.Publish(ctx, worker.Event{
		Type:      eventType,
		WorkerID:  w.ID,
		State:     w.State,
		Timestamp: time.Now(),
		Details:   details,
	})
}
