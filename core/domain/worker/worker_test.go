package worker_test

import (
	"testing"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

func TestWorkerState_String(t *testing.T) {
	tests := []struct {
		state  worker.State
		expect string
	}{
		{worker.StateStarting, "starting"},
		{worker.StateRunning, "running"},
		{worker.StateUnhealthy, "unhealthy"},
		{worker.StateRestarting, "restarting"},
		{worker.StateStopped, "stopped"},
		{worker.State("unknown"), "unknown"},
	}

	for _, tc := range tests {
		if got := string(tc.state); got != tc.expect {
			t.Errorf("expected %s, got %s", tc.expect, got)
		}
	}
}

func TestWorker_IsAlive(t *testing.T) {
	w := &worker.Worker{
		ID:     "worker1",
		State:  worker.StateRunning,
	}

	if !w.IsAlive() {
		t.Error("expected worker to be alive")
	}
}

func TestWorker_IsNotAlive_WhenStopped(t *testing.T) {
	w := &worker.Worker{
		ID:    "worker1",
		State: worker.StateStopped,
	}

	if w.IsAlive() {
		t.Error("expected worker to not be alive")
	}
}

func TestWorker_IsNotAlive_WhenStarting(t *testing.T) {
	w := &worker.Worker{
		ID:    "worker1",
		State: worker.StateStarting,
	}

	if w.IsAlive() {
		t.Error("expected worker to not be alive")
	}
}

func TestWorker_DefaultValues(t *testing.T) {
	w := worker.Worker{
		ID:     "worker1",
		State: worker.StateStarting,
	}

	if w.ID != "worker1" {
		t.Error("ID not set")
	}
	if w.RestartCount != 0 {
		t.Error("restart count should be 0")
	}
}