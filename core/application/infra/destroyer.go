package infra

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// Destroyer removes all managed resources in reverse dependency order.
type Destroyer struct {
	registry infra.ProviderRegistry
	backend  infra.Backend
}

// NewDestroyer creates a new Destroyer.
func NewDestroyer(registry infra.ProviderRegistry, backend infra.Backend) *Destroyer {
	return &Destroyer{
		registry: registry,
		backend:  backend,
	}
}

// Destroy removes all resources from the stack in reverse dependency order.
func (d *Destroyer) Destroy(ctx context.Context, stack *infra.Stack) error {
	// Load current state.
	currentState, err := d.backend.Get(ctx)
	if err != nil {
		return fmt.Errorf("infra destroy: load state: %w", err)
	}
	if currentState == nil || len(currentState.Resources) == 0 {
		return nil // nothing to destroy
	}

	// Get resources in reverse topological order.
	revSorted, err := stack.TopologicalSortReverse()
	if err != nil {
		return fmt.Errorf("infra destroy: topological sort: %w", err)
	}

	// Only destroy resources that exist in the current state.
	existing := make(map[infra.ResourceID]bool)
	for _, r := range currentState.Resources {
		existing[r.ID] = true
	}

	for _, r := range revSorted {
		if !existing[r.ID] {
			continue
		}

		provider, err := d.registry.Get(r.ProviderName)
		if err != nil {
			return fmt.Errorf("infra destroy: get provider %q for %q: %w",
				r.ProviderName, r.ID, err)
		}

		if err := provider.Delete(ctx, r); err != nil {
			return fmt.Errorf("infra destroy: delete %q: %w", r.ID, err)
		}
	}

	// Remove state after successful destroy.
	if err := d.backend.Delete(ctx); err != nil {
		return fmt.Errorf("infra destroy: delete state: %w", err)
	}

	return nil
}
