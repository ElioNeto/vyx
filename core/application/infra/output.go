package infra

import (
	"context"
	"fmt"
	"sort"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// OutputCollector aggregates output values from the current state.
type OutputCollector struct {
	backend infra.Backend
}

// NewOutputCollector creates a new OutputCollector.
func NewOutputCollector(backend infra.Backend) *OutputCollector {
	return &OutputCollector{
		backend: backend,
	}
}

// OutputEntry represents a single output value with metadata.
type OutputEntry struct {
	ResourceID infra.ResourceID `json:"resource_id"`
	ResourceType infra.ResourceType `json:"resource_type"`
	Name       string           `json:"name"`
	Value      string           `json:"value"`
}

// AllOutputs returns all outputs from the current state.
func (c *OutputCollector) AllOutputs(ctx context.Context) ([]OutputEntry, error) {
	state, err := c.backend.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("output collector: load state: %w", err)
	}
	if state == nil {
		return nil, nil
	}

	var entries []OutputEntry
	for _, r := range state.Resources {
		for name, value := range r.Outputs {
			entries = append(entries, OutputEntry{
				ResourceID:   r.ID,
				ResourceType: r.Type,
				Name:         name,
				Value:        value,
			})
		}
	}

	// Sort for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ResourceID != entries[j].ResourceID {
			return entries[i].ResourceID < entries[j].ResourceID
		}
		return entries[i].Name < entries[j].Name
	})

	return entries, nil
}

// GetOutput returns a specific output value by resource ID and output name.
// Uses the format "resource_id.output_name".
func (c *OutputCollector) GetOutput(ctx context.Context, ref string) (string, error) {
	parts := splitOutputRef(ref)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid output reference %q — use format: resource_id.output_name", ref)
	}

	resourceID := infra.ResourceID(parts[0])
	outputName := parts[1]

	state, err := c.backend.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("output collector: load state: %w", err)
	}
	if state == nil {
		return "", fmt.Errorf("no state found")
	}

	for _, r := range state.Resources {
		if r.ID == resourceID {
			if val, ok := r.Outputs[outputName]; ok {
				return val, nil
			}
			return "", fmt.Errorf("output %q not found on resource %q", outputName, resourceID)
		}
	}

	return "", fmt.Errorf("resource %q not found in state", resourceID)
}

// OutputsMap returns outputs grouped by resource ID.
func (c *OutputCollector) OutputsMap(ctx context.Context) (infra.ResourceOutputs, error) {
	state, err := c.backend.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("output collector: load state: %w", err)
	}
	if state == nil {
		return nil, nil
	}

	hasOutputs := false
	for _, r := range state.Resources {
		if len(r.Outputs) > 0 {
			hasOutputs = true
			break
		}
	}
	if !hasOutputs {
		return nil, nil
	}

	result := make(infra.ResourceOutputs)
	for _, r := range state.Resources {
		if len(r.Outputs) > 0 {
			result[r.ID] = r.Outputs
		}
	}
	return result, nil
}

// splitOutputRef splits "bucket.arn" into ["bucket", "arn"].
func splitOutputRef(ref string) []string {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '.' {
			return []string{ref[:i], ref[i+1:]}
		}
	}
	return nil
}
