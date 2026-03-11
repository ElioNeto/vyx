// Package worker defines the core domain entities and interfaces for worker management.
// This is the innermost layer of the Clean Architecture — no external dependencies.
package worker

import (
	"context"
	"time"
)

// State represents the lifecycle state of a worker process.
type State string

const (
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateUnhealthy  State = "unhealthy"
	StateRestarting State = "restarting"
	StateStopped    State = "stopped"
)

// Worker is the domain entity representing a managed worker process.
type Worker struct {
	ID             string
	Command        string
	Args           []string
	State          State
	RestartCount   int
	LastHeartbeat  time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsAlive returns true when the worker is in a healthy operational state.
func (w *Worker) IsAlive() bool {
	return w.State == StateRunning
}

// Repository defines persistence operations for workers.
// Implemented by the infrastructure layer.
type Repository interface {
	Save(ctx context.Context, w *Worker) error
	FindByID(ctx context.Context, id string) (*Worker, error)
	FindAll(ctx context.Context) ([]*Worker, error)
	Delete(ctx context.Context, id string) error
}

// Manager defines the orchestration contract for worker processes.
// Implemented by the infrastructure layer (process manager).
type Manager interface {
	Spawn(ctx context.Context, w *Worker) error
	Stop(ctx context.Context, id string) error
	StopAll(ctx context.Context) error
	SendHeartbeat(ctx context.Context, id string) error
}

// EventPublisher defines how domain events are emitted.
// Implemented by the infrastructure layer (logger, event bus, etc.).
type EventPublisher interface {
	Publish(ctx context.Context, event Event)
}
