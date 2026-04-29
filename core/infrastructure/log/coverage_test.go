package log

import (
	"strings"
	"testing"
	"time"

	dlog "github.com/ElioNeto/vyx/core/domain/log"
)

func TestAddSourceWithPrefix(t *testing.T) {
	m := New()
	sourceTag := "custom-source"
	r := strings.NewReader("Line 1\nLine 2\n")

	stop := m.AddSourceWithPrefix(sourceTag, r)
	defer stop()

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)

	entries := m.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}

	// Verify source tag is applied
	if entries[0].Source != sourceTag {
		t.Errorf("Source = %q, want %q", entries[0].Source, sourceTag)
	}
}

func TestScanSource(t *testing.T) {
	m := New()
	r := strings.NewReader("Test line 1\nTest line 2\n")

	stop := m.scanSource("test-worker", r, dlog.ParseEntry)
	defer stop()

	time.Sleep(50 * time.Millisecond)

	entries := m.Entries()
	if len(entries) < 2 {
		t.Fatalf("entries = %d, want at least 2", len(entries))
	}
}

func TestProcessScannerLine(t *testing.T) {
	m := New()

	// Test with empty line
	m.processScannerLine("worker1", dlog.ParseEntry, "")
	entries := m.Entries()
	if len(entries) != 0 {
		t.Errorf("empty line should not create entry, got %d entries", len(entries))
	}

	// Test with valid line
	m.processScannerLine("worker1", dlog.ParseEntry, "2024-01-01 INFO test message")
	entries = m.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
}

func TestContainsField(t *testing.T) {
	tests := []struct {
		name     string
		entry    dlog.Entry
		substr   string
		expected bool
	}{
		{"in_message", dlog.Entry{Message: "hello world"}, "world", true},
		{"not_in_message", dlog.Entry{Message: "hello"}, "world", false},
		{"in_raw", dlog.Entry{Raw: "raw data here"}, "data", true},
		{"in_fields", dlog.Entry{Fields: map[string]string{"field1": "value with keyword"}}, "keyword", true},
		{"empty", dlog.Entry{}, "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsField(tt.entry, tt.substr)
			if result != tt.expected {
				t.Errorf("containsField() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLevelGreaterOrEqual(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		min      string
		expected bool
	}{
		{"debug_vs_info", "DEBUG", "INFO", false},
		{"info_vs_info", "INFO", "INFO", true},
		{"warn_vs_info", "WARN", "INFO", true},
		{"error_vs_warn", "ERROR", "WARN", true},
		{"unknown_level", "TRACE", "INFO", true}, // Unknown levels pass through
		{"empty_min", "INFO", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := levelGreaterOrEqual(tt.level, tt.min)
			if result != tt.expected {
				t.Errorf("levelGreaterOrEqual(%q, %q) = %v, want %v", tt.level, tt.min, result, tt.expected)
			}
		})
	}
}

func TestFilterBySource_EdgeCases(t *testing.T) {
	m := Multiplexer{}
	entries := []dlog.Entry{
		{Source: "go:api"},
		{Source: "node:api"},
		{Source: "go:api"},
	}

	// Filter with empty string should return all
	filtered := m.FilterBySource(entries, "")
	if len(filtered) != 3 {
		t.Errorf("empty filter should return all, got %d", len(filtered))
	}

	// Filter with "ALL" should return all
	filtered = m.FilterBySource(entries, "ALL")
	if len(filtered) != 3 {
		t.Errorf("ALL filter should return all, got %d", len(filtered))
	}
}

func TestFilterByLevel_EdgeCases(t *testing.T) {
	m := Multiplexer{}
	entries := []dlog.Entry{
		{Level: "DEBUG"},
		{Level: "INFO"},
		{Level: "WARN"},
		{Level: "ERROR"},
	}

	// Filter with empty string should return all
	filtered := m.FilterByLevel(entries, "")
	if len(filtered) != 4 {
		t.Errorf("empty filter should return all, got %d", len(filtered))
	}

	// Filter with "ALL" should return all
	filtered = m.FilterByLevel(entries, "ALL")
	if len(filtered) != 4 {
		t.Errorf("ALL filter should return all, got %d", len(filtered))
	}
}

func TestFilterByCorrelationID_EdgeCases(t *testing.T) {
	m := Multiplexer{}
	entries := []dlog.Entry{
		{CorrelationID: "req-123"},
		{CorrelationID: "req-456"},
		{CorrelationID: ""},
	}

	// Empty correlation ID should return all
	filtered := m.FilterByCorrelationID(entries, "")
	if len(filtered) != 3 {
		t.Errorf("empty corr ID should return all, got %d", len(filtered))
	}
}
