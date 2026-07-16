package infra

import "fmt"

// ─── Sentinel Errors ─────────────────────────────────────────────────────

// ErrResourceNotFound is returned when a resource is not found in the state.
type ErrResourceNotFound struct {
	ResourceID ResourceID
}

func (e *ErrResourceNotFound) Error() string {
	return fmt.Sprintf("resource %q not found", e.ResourceID)
}

// ErrProviderNotFound is returned when a provider is not registered.
type ErrProviderNotFound struct {
	ProviderID ProviderID
}

func (e *ErrProviderNotFound) Error() string {
	return fmt.Sprintf("provider %q not found — is it registered?", e.ProviderID)
}

// ErrProviderAlreadyRegistered is returned when a provider is already registered.
type ErrProviderAlreadyRegistered struct {
	ProviderID ProviderID
}

func (e *ErrProviderAlreadyRegistered) Error() string {
	return fmt.Sprintf("provider %q is already registered", e.ProviderID)
}

// ErrValidation is returned when a resource fails validation.
type ErrValidation struct {
	ResourceID ResourceID
	Message    string
}

func (e *ErrValidation) Error() string {
	return fmt.Sprintf("resource %q validation failed: %s", e.ResourceID, e.Message)
}

// ErrCycleDetected is returned when the dependency graph has a cycle.
type ErrCycleDetected struct {
	StackName string
}

func (e *ErrCycleDetected) Error() string {
	return fmt.Sprintf("dependency cycle detected in stack %q", e.StackName)
}

// ErrLockAcquisition is returned when a lock cannot be acquired.
type ErrLockAcquisition struct {
	Info LockInfo
	Err  error
}

func (e *ErrLockAcquisition) Error() string {
	return fmt.Sprintf("failed to acquire lock for %s (operation: %s): %v", e.Info.ID, e.Info.Operation, e.Err)
}

func (e *ErrLockAcquisition) Unwrap() error { return e.Err }

// ErrStateSerialMismatch is returned when applying a stale state.
type ErrStateSerialMismatch struct {
	Expected uint64
	Actual   uint64
}

func (e *ErrStateSerialMismatch) Error() string {
	return fmt.Sprintf("state serial mismatch: expected %d, got %d — state was modified concurrently", e.Expected, e.Actual)
}

// ErrProviderFailed is returned when a provider operation fails.
type ErrProviderFailed struct {
	ProviderID ProviderID
	ResourceID ResourceID
	Operation  string
	Err        error
}

func (e *ErrProviderFailed) Error() string {
	return fmt.Sprintf("provider %q failed to %s resource %q: %v", e.ProviderID, e.Operation, e.ResourceID, e.Err)
}

func (e *ErrProviderFailed) Unwrap() error { return e.Err }
