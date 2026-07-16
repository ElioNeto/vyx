package infra

import (
	"context"
	"time"
)

// StateID is a unique identifier for an infrastructure state snapshot.
type StateID string

// State represents a complete snapshot of managed infrastructure.
type State struct {
	ID          StateID      `json:"id"`
	Serial      uint64       `json:"serial"`
	Version     string       `json:"version"` // vyx framework version
	Resources   []*Resource  `json:"resources"`
	ProviderIDs []ProviderID `json:"providers"`
	Metadata    StateMetadata `json:"metadata"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// StateMetadata holds informational metadata about the state.
type StateMetadata struct {
	StackName   string `json:"stack_name"`
	Description string `json:"description,omitempty"`
}

// NewState creates a new empty State with serial 1.
func NewState(stackName string) *State {
	now := Now()
	return &State{
		ID:        StateID(stackName + "-" + now.Format("20060102150405")),
		Serial:    1,
		Version:   "0.2.0",
		Resources: []*Resource{},
		Metadata: StateMetadata{
			StackName: stackName,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// LockInfo describes a lock holder for concurrency control.
type LockInfo struct {
	ID        string    `json:"id"`
	Operation string    `json:"operation"` // "plan", "apply", "destroy"
	Who       string    `json:"who"`       // hostname or user
	CreatedAt time.Time `json:"created_at"`
	TTL       string    `json:"ttl,omitempty"` // e.g. "30s"
}

// Backend is the interface for state persistence.
type Backend interface {
	// Init initializes the backend (creates bucket, directory, etc.).
	Init(ctx context.Context) error

	// Lock acquires a lock to prevent concurrent operations.
	Lock(ctx context.Context, info LockInfo) error

	// Unlock releases a previously acquired lock.
	Unlock(ctx context.Context, info LockInfo) error

	// Get retrieves the current state.
	Get(ctx context.Context) (*State, error)

	// Put persists a new state snapshot.
	Put(ctx context.Context, state *State) error

	// Delete removes the state entirely (after destroy).
	Delete(ctx context.Context) error
}
