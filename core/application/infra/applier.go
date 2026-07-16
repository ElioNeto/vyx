package infra

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// Applier executes the planned changes in the correct order.
type Applier struct {
	registry infra.ProviderRegistry
	backend  infra.Backend
}

// NewApplier creates a new Applier.
func NewApplier(registry infra.ProviderRegistry, backend infra.Backend) *Applier {
	return &Applier{
		registry: registry,
		backend:  backend,
	}
}

// Apply executes all changes described in the plan, respecting the resource order.
func (a *Applier) Apply(ctx context.Context, plan *infra.PlanResult, stack *infra.Stack) (*infra.State, error) {
	if !plan.HasChanges() {
		return nil, nil
	}

	// Load current state as base.
	currentState, err := a.backend.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("infra apply: load state: %w", err)
	}

	// Build a mutable map of resources.
	resourceMap := make(map[infra.ResourceID]*infra.Resource)
	if currentState != nil {
		for _, r := range currentState.Resources {
			resourceMap[r.ID] = r.Clone()
		}
	}

	// Execute changes in topological order.
	sorted, err := stack.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("infra apply: topological sort: %w", err)
	}

	desiredByID := make(map[infra.ResourceID]*infra.Resource)
	for _, r := range sorted {
		desiredByID[r.ID] = r
	}

	for _, change := range plan.Changes {
		provider, err := a.registry.Get(change.ProviderName)
		if err != nil {
			return nil, fmt.Errorf("infra apply: get provider %q: %w", change.ProviderName, err)
		}

		switch change.ChangeType {
		case infra.ChangeCreate:
			desired, ok := desiredByID[change.ResourceID]
			if !ok {
				return nil, fmt.Errorf("infra apply: resource %q not found in stack", change.ResourceID)
			}
			created, err := provider.Create(ctx, desired)
			if err != nil {
				return nil, fmt.Errorf("infra apply: create %q: %w", change.ResourceID, err)
			}
			resourceMap[created.ID] = created

		case infra.ChangeUpdate:
			desired, ok := desiredByID[change.ResourceID]
			if !ok {
				return nil, fmt.Errorf("infra apply: resource %q not found in stack", change.ResourceID)
			}
			current := resourceMap[change.ResourceID]
			updated, err := provider.Update(ctx, desired, current)
			if err != nil {
				return nil, fmt.Errorf("infra apply: update %q: %w", change.ResourceID, err)
			}
			resourceMap[updated.ID] = updated

		case infra.ChangeDelete:
			current := resourceMap[change.ResourceID]
			if err := provider.Delete(ctx, current); err != nil {
				return nil, fmt.Errorf("infra apply: delete %q: %w", change.ResourceID, err)
			}
			delete(resourceMap, change.ResourceID)

		case infra.ChangeNoop:
			// Nothing to do.
		}
	}

	// Build the new state.
	newState := a.buildState(resourceMap, stack)
	if err := a.backend.Put(ctx, newState); err != nil {
		return nil, fmt.Errorf("infra apply: persist state: %w", err)
	}

	return newState, nil
}

// buildState constructs a State from the resource map.
func (a *Applier) buildState(resourceMap map[infra.ResourceID]*infra.Resource, stack *infra.Stack) *infra.State {
	resources := make([]*infra.Resource, 0, len(resourceMap))
	providerSet := make(map[infra.ProviderID]bool)

	for _, r := range resourceMap {
		resources = append(resources, r)
		providerSet[r.ProviderName] = true
	}

	providerIDs := make([]infra.ProviderID, 0, len(providerSet))
	for pid := range providerSet {
		providerIDs = append(providerIDs, pid)
	}

	state := infra.NewState(stack.Name)
	state.Resources = resources
	state.ProviderIDs = providerIDs
	return state
}
