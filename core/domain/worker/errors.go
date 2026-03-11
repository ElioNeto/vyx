package worker

import "errors"

var (
	ErrNotFound        = errors.New("worker not found")
	ErrAlreadyRunning  = errors.New("worker is already running")
	ErrSpawnFailed     = errors.New("failed to spawn worker process")
	ErrStopTimeout     = errors.New("worker did not stop within the allowed timeout")
	ErrInvalidCommand  = errors.New("worker command is empty or invalid")
)
