// Package log defines the structured log entry domain type and source metadata.
package log

// Source identifies the origin of a log entry and provides display metadata.
type Source string

const (
	SourceCore   Source = "CORE"
	SourceGo     Source = "go"
	SourceNode   Source = "node"
	SourcePython Source = "python"
)

// ParseSource infers the Source from a worker ID.
// "node:api" → SourceNode, "go:api" → SourceGo, "python:ml" → SourcePython.
func ParseSource(workerID string) Source {
	switch {
	case workerID == "":
		return SourceCore
	case len(workerID) >= 4 && workerID[:5] == "node:":
		return SourceNode
	case len(workerID) >= 2 && workerID[:3] == "go:":
		return SourceGo
	case len(workerID) >= 6 && workerID[:7] == "python:":
		return SourcePython
	default:
		return SourceGo // default to Go for unknown workers
	}
}

// Tag returns the display tag for this source (e.g., "CORE", "node:api").
func (s Source) Tag(workerID string) string {
	if s == SourceCore {
		return "CORE"
	}
	return workerID
}
