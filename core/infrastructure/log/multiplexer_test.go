package log

import (
	"strings"
	"testing"
	"time"

	dlog "github.com/ElioNeto/vyx/core/domain/log"
)

func TestMultiplexerPush(t *testing.T) {
	m := New(10)
	now := time.Now()
	m.Push(dlog.Entry{Timestamp: now, Message: "hello"})

	entries := m.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Message != "hello" {
		t.Fatalf("message = %q, want %q", entries[0].Message, "hello")
	}
}

func TestMultiplexerRingBuffer(t *testing.T) {
	m := New(3)
	for i := 0; i < 5; i++ {
		m.Push(dlog.Entry{Timestamp: time.Now(), Message: string(rune('a' + i))})
	}
	entries := m.Entries()
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	// Should see the last 3 entries: c, d, e
	if entries[0].Message != "c" {
		t.Fatalf("first = %q, want %q", entries[0].Message, "c")
	}
	if entries[2].Message != "e" {
		t.Fatalf("last = %q, want %q", entries[2].Message, "e")
	}
}

func TestMultiplexerSubscribe(t *testing.T) {
	m := New()
	ch := m.Subscribe()
	defer m.Unsubscribe(ch)

	m.Push(dlog.Entry{Timestamp: time.Now(), Message: "test"})

	select {
	case e := <-ch:
		if e.Message != "test" {
			t.Fatalf("received message = %q, want %q", e.Message, "test")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for entry on subscribe channel")
	}
}

func TestAddSourceRawFallback(t *testing.T) {
	m := New()
	r := strings.NewReader("Hello world\nprint(\"test\")\n")
	stop := m.AddSource("python:ml", r)
	defer stop()

	time.Sleep(50 * time.Millisecond)

	entries := m.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Raw != "Hello world" {
		t.Fatalf("raw = %q, want %q", entries[0].Raw, "Hello world")
	}
	if entries[1].Raw != "print(\"test\")" {
		t.Fatalf("raw = %q, want %q", entries[1].Raw, "print(\"test\")")
	}
}

func TestFilterByLevel(t *testing.T) {
	m := Multiplexer{}
	entries := []dlog.Entry{
		{Level: "DEBUG"},
		{Level: "INFO"},
		{Level: "WARN"},
		{Level: "ERROR"},
	}
	filtered := m.FilterByLevel(entries, "WARN")
	if len(filtered) != 2 {
		t.Fatalf("filtered = %d, want 2", len(filtered))
	}
	if filtered[0].Level != "WARN" || filtered[1].Level != "ERROR" {
		t.Fatalf("levels = %q, %q, want WARN, ERROR", filtered[0].Level, filtered[1].Level)
	}
}

func TestFilterBySource(t *testing.T) {
	m := Multiplexer{}
	entries := []dlog.Entry{
		{Source: "go:api"},
		{Source: "node:api"},
		{Source: "go:api"},
	}
	filtered := m.FilterBySource(entries, "go:api")
	if len(filtered) != 2 {
		t.Fatalf("filtered = %d, want 2", len(filtered))
	}
}

func TestFilterByCorrelationID(t *testing.T) {
	m := Multiplexer{}
	entries := []dlog.Entry{
		{CorrelationID: "req-123", Message: "handler called"},
		{CorrelationID: "req-456", Message: "another"},
		{Message: "no correlation"},
		{Message: "matching req-123 in text"},
	}
	filtered := m.FilterByCorrelationID(entries, "req-123")
	if len(filtered) != 2 {
		t.Fatalf("filtered = %d, want 2", len(filtered))
	}
}
