package log

import (
	"testing"
	"time"
)

func TestParseSource(t *testing.T) {
	cases := []struct {
		workerID string
		want     Source
	}{
		{"", SourceCore},
		{"node:api", SourceNode},
		{"go:api", SourceGo},
		{"python:ml", SourcePython},
		{"unknown:worker", SourceGo},
	}
	for _, tc := range cases {
		if got := ParseSource(tc.workerID); got != tc.want {
			t.Errorf("ParseSource(%q) = %q, want %q", tc.workerID, got, tc.want)
		}
	}
}

func TestSourceTag(t *testing.T) {
	cases := []struct {
		source   Source
		workerID string
		want     string
	}{
		{SourceCore, "", "CORE"},
		{SourceNode, "node:api", "node:api"},
		{SourceGo, "go:api", "go:api"},
		{SourcePython, "python:ml", "python:ml"},
	}
	for _, tc := range cases {
		if got := tc.source.Tag(tc.workerID); got != tc.want {
			t.Errorf("Source(%q).Tag(%q) = %q, want %q", tc.source, tc.workerID, got, tc.want)
		}
	}
}

func TestParseEntryJSON(t *testing.T) {
	line := `{"timestamp":"2024-01-01T12:00:00Z","source":"CORE","level":"INFO","message":"server started"}`
	e := ParseEntry("", line)
	if e.Level != "INFO" {
		t.Errorf("level = %q, want %q", e.Level, "INFO")
	}
	if e.Message != "server started" {
		t.Errorf("message = %q, want %q", e.Message, "server started")
	}
	if e.Timestamp != time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC) {
		t.Errorf("timestamp = %v, want %v", e.Timestamp, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	}
}

func TestParseEntryRawFallback(t *testing.T) {
	line := "Hello from Python print()"
	e := ParseEntry("python:ml", line)
	if e.Level != "INFO" {
		t.Errorf("level = %q, want %q", e.Level, "INFO")
	}
	if e.Raw != line {
		t.Errorf("raw = %q, want %q", e.Raw, line)
	}
	if e.Source != "python:ml" {
		t.Errorf("source = %q, want %q", e.Source, "python:ml")
	}
}

func TestParseEntryCorrelationID(t *testing.T) {
	line := `{"timestamp":"2024-01-01T12:00:00Z","level":"INFO","message":"request","correlation_id":"req-123","source":"go:api"}`
	e := ParseEntry("go:api", line)
	if e.CorrelationID != "req-123" {
		t.Errorf("correlation_id = %q, want %q", e.CorrelationID, "req-123")
	}
}

func TestEntryTime(t *testing.T) {
	e := Entry{
		Timestamp: time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
	}
	if got := e.Time(); got != "12:30:45" {
		t.Errorf("Time() = %q, want %q", got, "12:30:45")
	}
}
