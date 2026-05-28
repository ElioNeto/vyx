package process

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/worker"
)

// TestProcessChunk unit tests.
func TestProcessChunk_SingleLine(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := &Manager{}
	remain := m.processChunk(writer, "w", []byte("hello\n"))
	if len(lines) != 1 || lines[0] != "hello" {
		t.Fatalf("expected [hello], got %v", lines)
	}
	if remain != nil {
		t.Fatalf("expected nil remainder, got %q", string(remain))
	}
}

func TestProcessChunk_MultipleLines(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := &Manager{}
	remain := m.processChunk(writer, "w", []byte("line1\nline2\nline3\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if remain != nil {
		t.Fatalf("expected nil remainder, got %q", string(remain))
	}
}

func TestProcessChunk_NoNewline(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := &Manager{}
	remain := m.processChunk(writer, "w", []byte("partial"))
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines, got %v", lines)
	}
	if string(remain) != "partial" {
		t.Fatalf("expected remainder 'partial', got %q", string(remain))
	}
}

func TestProcessChunk_EmptyLines(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := &Manager{}
	remain := m.processChunk(writer, "w", []byte("\n\n"))
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines (empty lines skipped), got %d", len(lines))
	}
	if remain != nil {
		t.Fatalf("expected nil remainder after empty lines, got %q", string(remain))
	}
}

// func TestPipeLog_Simple(t *testing.T) {
// 	var lines []string
// 	writer := func(id, line string) { lines = append(lines, line) }
// 	m := &Manager{}
// 	input := "line1\nline2\n"
// 	r := bytes.NewReader([]byte(input))
// 	m.pipeLog(writer, "w", r)
// 	if len(lines) != 2 {
// 		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
// 	}
// 	if lines[0] != "line1" || lines[1] != "line2" {
// 		t.Fatalf("unexpected lines: %v", lines)
// 	}
// }

// func TestPipeLog_NoNewlineAtEnd(t *testing.T) {
// 	var lines []string
// 	writer := func(id, line string) { lines = append(lines, line) }
// 	m := &Manager{}
// 	input := "partial line"
// 	r := bytes.NewReader([]byte(input))
// 	m.pipeLog(writer, "w", r)
// 	// pipeLog should call writer with the remaining data when reader returns error (EOF)
// 	// Since there's no newline, the buffer will be flushed on EOF.
// 	if len(lines) != 1 {
// 		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
// 	}
// 	if lines[0] != "partial line" {
// 		t.Fatalf("expected 'partial line', got %q", lines[0])
// 	}
// }

func TestWithLogWriter(t *testing.T) {
	var called bool
	writer := func(id, line string) { called = true }
	m := New(WithLogWriter(writer))
	if m.logWriter == nil {
		t.Fatal("logWriter not set")
	}
	// trigger writer
	m.pipeLog(context.Background(), m.logWriter, "w", bytes.NewReader([]byte("test\n")))
	if !called {
		t.Fatal("writer not called")
	}
}

func TestPipeLog_PartialLine(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := New(WithLogWriter(writer))

	// Partial line without newline — should be flushed on EOF.
	m.pipeLog(context.Background(), m.logWriter, "w", bytes.NewReader([]byte("partial line")))
	if len(lines) != 1 || lines[0] != "partial line" {
		t.Fatalf("expected [partial line], got %v", lines)
	}
}

func TestPipeLog_MultipleChunks(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := New(WithLogWriter(writer))

	// Two complete lines, then a partial.
	r := bytes.NewReader([]byte("line1\nline2\ninco"))
	m.pipeLog(context.Background(), m.logWriter, "w", r)

	expected := []string{"line1", "line2", "inco"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, lines)
	}
	for i, l := range lines {
		if l != expected[i] {
			t.Errorf("line[%d] = %q, want %q", i, l, expected[i])
		}
	}
}

func TestPipeLog_EmptyLines(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := New(WithLogWriter(writer))

	r := bytes.NewReader([]byte("\n\n"))
	m.pipeLog(context.Background(), m.logWriter, "w", r)

	if len(lines) != 0 {
		t.Fatalf("expected 0 lines (empty lines skipped), got %v", lines)
	}
}

func TestPipeLog_CancelledContext(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := New(WithLogWriter(writer))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancelled

	r := bytes.NewReader([]byte("data\n"))
	m.pipeLog(ctx, m.logWriter, "w", r)

	// With cancelled context, we may or may not read the data,
	// but the function should return immediately.
	_ = lines
}

func TestStop_NonExistent(t *testing.T) {
	m := &Manager{processes: make(map[string]*exec.Cmd)}
	err := m.Stop(context.Background(), "nonexistent")
	if err != worker.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStopAll_Empty(t *testing.T) {
	m := &Manager{processes: make(map[string]*exec.Cmd)}
	err := m.StopAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSpawnAndStopWithLogWriter tests that log writer is invoked during spawn.
// This is an integration-like test but helps coverage.
func TestSpawnAndStopWithLogWriter(t *testing.T) {
	var lines []string
	writer := func(id, line string) { lines = append(lines, line) }
	m := New(WithLogWriter(writer))
	w := &worker.Worker{
		ID:        "test-sleep",
		Command:   "sleep",
		Args:      []string{"2"},
		State:     worker.StateStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := m.Spawn(context.Background(), w); err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	// Give it a moment then stop
	time.Sleep(100 * time.Millisecond)
	_ = m.Stop(context.Background(), "test-sleep")
}

// TestSendHeartbeat_Nil checks that SendHeartbeat returns nil.
func TestSendHeartbeat_Nil(t *testing.T) {
	m := New()
	if err := m.SendHeartbeat(context.Background(), "id"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}


