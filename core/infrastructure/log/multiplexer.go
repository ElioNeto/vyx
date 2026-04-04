// Package log provides a log multiplexer that aggregates structured log entries
// from multiple sources into a single, thread-safe ring buffer suitable for TUI display.
package log

import (
	"bufio"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ElioNeto/vyx/core/domain/log"
)

// Multiplexer aggregates log entries from multiple sources into a shared ring buffer.
// It provides a subscription channel for TUI consumers.
type Multiplexer struct {
	mu      sync.RWMutex
	entries []log.Entry
	size    int
	head    int // index of oldest entry
	count   int // total entries stored

	subsMu sync.RWMutex
	subs   []chan log.Entry
}

const defaultBufferSize = 5000

// New creates a Multiplexer with the given buffer capacity.
func New(capacity ...int) *Multiplexer {
	cap := defaultBufferSize
	if len(capacity) > 0 && capacity[0] > 0 {
		cap = capacity[0]
	}
	return &Multiplexer{
		entries: make([]log.Entry, cap),
		size:    cap,
		subs:    make([]chan log.Entry, 0),
	}
}

// Push adds a log entry to the ring buffer and notifies subscribers.
func (m *Multiplexer) Push(entry log.Entry) {
	m.mu.Lock()
	m.entries[m.head] = entry
	m.head = (m.head + 1) % m.size
	if m.count < m.size {
		m.count++
	}
	m.mu.Unlock()

	m.notify(entry)
}

// Entries returns all buffered entries in chronological order.
func (m *Multiplexer) Entries() []log.Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]log.Entry, m.count)
	start := (m.head - m.count + m.size) % m.size
	for i := 0; i < m.count; i++ {
		out[i] = m.entries[(start+i)%m.size]
	}
	return out
}

// AddSource reads from an io.Reader line-by-line and pushes parsed entries.
// Each line is parsed via domain/log.ParseEntry with the given workerID.
// Returns a stop function to halt the goroutine.
func (m *Multiplexer) AddSource(workerID string, r io.Reader) func() {
	stopCh := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		for {
			select {
			case <-stopCh:
				return
			default:
				if scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						continue
					}
					entry := log.ParseEntry(workerID, line)
					if entry.Source == "" {
						entry.Source = workerID
					}
					m.Push(entry)
				}
				if err := scanner.Err(); err != nil {
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	return func() { close(stopCh) }
}

// AddSourceWithPrefix is like AddSource but prepends a source tag for raw lines.
func (m *Multiplexer) AddSourceWithPrefix(sourceTag string, r io.Reader) func() {
	stopCh := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		for {
			select {
			case <-stopCh:
				return
			default:
				if scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						continue
					}
					entry := log.ParseEntry("", line)
					if entry.Source == "" {
						entry.Source = sourceTag
					}
					m.Push(entry)
				}
				if err := scanner.Err(); err != nil {
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	return func() { close(stopCh) }
}

// Subscribe returns a channel that receives new entries as they are pushed.
// The channel has a buffer of 256 entries. Call Unsubscribe when done.
func (m *Multiplexer) Subscribe() chan log.Entry {
	ch := make(chan log.Entry, 256)
	m.subsMu.Lock()
	m.subs = append(m.subs, ch)
	m.subsMu.Unlock()
	return ch
}

// Unsubscribe removes the channel from the subscriber list.
func (m *Multiplexer) Unsubscribe(ch chan log.Entry) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	for i, sub := range m.subs {
		if sub == ch {
			m.subs = append(m.subs[:i], m.subs[i+1:]...)
			close(ch)
			return
		}
	}
}

func (m *Multiplexer) notify(entry log.Entry) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	for _, ch := range m.subs {
		select {
		case ch <- entry:
		default:
			// Subscriber is slow — drop to avoid blocking the multiplexer.
		}
	}
}

// FilterBySource returns entries matching the given source tags.
// An empty or "ALL" filter returns all entries.
func (m *Multiplexer) FilterBySource(entries []log.Entry, tag string) []log.Entry {
	if tag == "" || tag == "ALL" {
		return entries
	}
	out := make([]log.Entry, 0, len(entries))
	for _, e := range entries {
		if e.Source == tag {
			out = append(out, e)
		}
	}
	return out
}

// FilterByLevel returns entries at or above the given level.
// Levels ordered: DEBUG < INFO < WARN < ERROR.
func (m *Multiplexer) FilterByLevel(entries []log.Entry, minLevel string) []log.Entry {
	if minLevel == "" || minLevel == "ALL" {
		return entries
	}
	out := make([]log.Entry, 0, len(entries))
	for _, e := range entries {
		if levelGreaterOrEqual(e.Level, minLevel) {
			out = append(out, e)
		}
	}
	return out
}

// FilterByCorrelationID returns entries matching the given correlation ID.
func (m *Multiplexer) FilterByCorrelationID(entries []log.Entry, corrID string) []log.Entry {
	if corrID == "" {
		return entries
	}
	out := make([]log.Entry, 0, len(entries))
	for _, e := range entries {
		if e.CorrelationID == corrID || containsField(e, corrID) {
			out = append(out, e)
		}
	}
	return out
}

func containsField(e log.Entry, substr string) bool {
	if strings.Contains(e.Message, substr) {
		return true
	}
	if strings.Contains(e.Raw, substr) {
		return true
	}
	for _, v := range e.Fields {
		if strings.Contains(v, substr) {
			return true
		}
	}
	return false
}

var levelOrder = map[string]int{
	"DEBUG": 0,
	"INFO":  1,
	"WARN":  2,
	"ERROR": 3,
}

func levelGreaterOrEqual(level, min string) bool {
	l, ok := levelOrder[strings.ToUpper(level)]
	if !ok {
		return true // unknown levels pass through
	}
	m, ok := levelOrder[strings.ToUpper(min)]
	if !ok {
		return true
	}
	return l >= m
}
