// Package infra implements the use cases for infrastructure management:
// plan, apply, destroy, and graph generation.
package infra

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// Planner computes the diff between the desired state (stack) and the current state (backend).
type Planner struct {
	registry infra.ProviderRegistry
	backend  infra.Backend
}

// NewPlanner creates a new Planner.
func NewPlanner(registry infra.ProviderRegistry, backend infra.Backend) *Planner {
	return &Planner{
		registry: registry,
		backend:  backend,
	}
}

// Plan generates a complete PlanResult by comparing desired resources against current state.
func (p *Planner) Plan(ctx context.Context, stack *infra.Stack) (*infra.PlanResult, error) {
	// Load current state from backend.
	currentState, err := p.backend.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("infra plan: load state: %w", err)
	}

	serial := uint64(0)
	if currentState != nil {
		serial = currentState.Serial
	}

	// Build a lookup of current resources by ID.
	currentResources := make(map[infra.ResourceID]*infra.Resource)
	if currentState != nil {
		for _, r := range currentState.Resources {
			currentResources[r.ID] = r
		}
	}

	var changes []*infra.ResourceChange

	// Sort resources topologically so plan order respects dependencies.
	sorted, err := stack.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("infra plan: topological sort: %w", err)
	}

	// Plan each desired resource.
	for _, desired := range sorted {
		provider, err := p.registry.Get(desired.ProviderName)
		if err != nil {
			return nil, fmt.Errorf("infra plan: get provider %q for resource %q: %w",
				desired.ProviderName, desired.ID, err)
		}

		current := currentResources[desired.ID]
		change, err := provider.Plan(ctx, desired, current)
		if err != nil {
			return nil, fmt.Errorf("infra plan: %s/%s: %w",
				desired.ProviderName, desired.ID, err)
		}
		changes = append(changes, change)
	}

	// Detect resources in current state but not in desired stack (delete).
	desiredIDs := make(map[infra.ResourceID]bool)
	for _, r := range stack.Resources {
		desiredIDs[r.ID] = true
	}
	for id, current := range currentResources {
		if !desiredIDs[id] {
			changes = append(changes, &infra.ResourceChange{
				ResourceID:   id,
				ChangeType:   infra.ChangeDelete,
				ResourceType: current.Type,
				ProviderName: current.ProviderName,
			})
		}
	}

	return infra.NewPlanResult(stack.Name, serial, changes), nil
}
