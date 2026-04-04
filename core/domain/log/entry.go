package log

import (
	"encoding/json"
	"time"
)

// Entry is the canonical structured log representation consumed by the TUI.
type Entry struct {
	Timestamp     time.Time         `json:"timestamp"`
	Source        string            `json:"source"`         // worker ID or "CORE"
	Level         string            `json:"level"`          // DEBUG, INFO, WARN, ERROR
	Message       string            `json:"message"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Fields        map[string]string `json:"fields,omitempty"`
	Raw           string            `json:"-"`              // original line for non-JSON fallback
}

// Time returns the formatted time for display.
func (e Entry) Time() string {
	return e.Timestamp.Format("15:04:05")
}

// ParseEntry attempts to parse a structured JSON log line into an Entry.
// If the line is not valid JSON, it falls back to a raw text entry with the
// given source and INFO level.
func ParseEntry(workerID string, line string) Entry {
	var entry Entry
	if err := json.Unmarshal([]byte(line), &entry); err == nil && !entry.Timestamp.IsZero() {
		return entry
	}

	// Fallback: treat as raw text (e.g., Python print, Node console.log)
	return Entry{
		Timestamp: time.Now(),
		Source:    workerID,
		Level:     "INFO",
		Raw:       line,
	}
}
