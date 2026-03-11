package worker

import "time"

// EventType identifies the kind of lifecycle transition that occurred.
type EventType string

const (
	EventSpawned     EventType = "worker.spawned"
	EventRunning     EventType = "worker.running"
	EventUnhealthy   EventType = "worker.unhealthy"
	EventRestarting  EventType = "worker.restarting"
	EventStopped     EventType = "worker.stopped"
	EventHeartbeat   EventType = "worker.heartbeat"
)

// Event carries information about a worker lifecycle transition.
type Event struct {
	Type      EventType
	WorkerID  string
	State     State
	Timestamp time.Time
	Details   string
}
