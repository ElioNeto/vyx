package infra

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// Importer imports existing cloud resources into the state.
type Importer struct {
	registry infra.ProviderRegistry
	backend  infra.Backend
}

// NewImporter creates a new Importer.
func NewImporter(registry infra.ProviderRegistry, backend infra.Backend) *Importer {
	return &Importer{
		registry: registry,
		backend:  backend,
	}
}

// ImportResult describes the outcome of an import operation.
type ImportResult struct {
	ResourceID   infra.ResourceID   `json:"resource_id"`
	ResourceType infra.ResourceType `json:"resource_type"`
	ProviderName infra.ProviderID   `json:"provider"`
	Found        bool               `json:"found"`
	OutputCount  int                `json:"output_count"`
}

// Import reads an existing resource from the cloud and adds it to the state.
// The resourceID is the vyx identifier, cloudID is the provider-specific identifier.
func (im *Importer) Import(ctx context.Context, resourceType infra.ResourceType, resourceID infra.ResourceID, cloudID string) (*ImportResult, error) {
	// Find which provider handles this resource type
	var matchedProvider infra.Provider
	for _, pid := range im.registry.List() {
		p, err := im.registry.Get(pid)
		if err != nil {
			continue
		}
		for _, cap := range p.Capabilities() {
			if cap.Type == resourceType {
				matchedProvider = p
				break
			}
		}
		if matchedProvider != nil {
			break
		}
	}

	if matchedProvider == nil {
		return nil, fmt.Errorf("import: no provider found for resource type %q", resourceType)
	}

	// Create a temporary resource to read from the cloud.
	tmpResource := infra.NewResource(resourceID, resourceType, matchedProvider.ID())
	tmpResource.Properties = map[string]any{
		"id": cloudID,
	}

	// Read the resource from the cloud.
	existing, err := matchedProvider.Read(ctx, tmpResource)
	if err != nil {
		return nil, fmt.Errorf("import: read %q (%s) from %s: %w", resourceID, cloudID, matchedProvider.ID(), err)
	}

	// Override the resource ID with the vyx-provided identifier.
	existing.ID = resourceID
	currentState, err := im.backend.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("import: load state: %w", err)
	}
	if currentState == nil {
		currentState = infra.NewState("default")
	}

	// Check for duplicates
	for _, r := range currentState.Resources {
		if r.ID == resourceID {
			return nil, fmt.Errorf("import: resource %q already exists in state", resourceID)
		}
	}

	currentState.Resources = append(currentState.Resources, existing)
	currentState.Serial++

	if err := im.backend.Put(ctx, currentState); err != nil {
		return nil, fmt.Errorf("import: persist state: %w", err)
	}

	return &ImportResult{
		ResourceID:   existing.ID,
		ResourceType: existing.Type,
		ProviderName: matchedProvider.ID(),
		Found:        true,
		OutputCount:  len(existing.Outputs),
	}, nil
}

// DiscoverResources lists all resources of a given type that exist in the cloud
// but are not yet managed by the state. Requires provider support for listing.
func (im *Importer) DiscoverResources(ctx context.Context, resourceType infra.ResourceType) ([]*infra.Resource, error) {
	var results []*infra.Resource

	for _, pid := range im.registry.List() {
		p, err := im.registry.Get(pid)
		if err != nil {
			continue
		}

		// Check if the provider supports this resource type
		supported := false
		for _, cap := range p.Capabilities() {
			if cap.Type == resourceType {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}

		// Try provider-specific listing via Lister interface
		if lister, ok := p.(interface {
			List(ctx context.Context, resourceType infra.ResourceType) ([]*infra.Resource, error)
		}); ok {
			resources, err := lister.List(ctx, resourceType)
			if err != nil {
				continue
			}
			results = append(results, resources...)
		}
	}

	// Filter out resources already managed in state
	currentState, err := im.backend.Get(ctx)
	if err != nil || currentState == nil {
		return results, nil
	}

	managed := make(map[infra.ResourceID]bool)
	for _, r := range currentState.Resources {
		managed[r.ID] = true
	}

	var unmanaged []*infra.Resource
	for _, r := range results {
		if !managed[r.ID] {
			unmanaged = append(unmanaged, r)
		}
	}

	return unmanaged, nil
}

// StateManager handles state-level operations.
type StateManager struct {
	backend infra.Backend
}

// NewStateManager creates a new StateManager.
func NewStateManager(backend infra.Backend) *StateManager {
	return &StateManager{backend: backend}
}

// ListResources returns all resource IDs in the current state.
func (sm *StateManager) ListResources(ctx context.Context) ([]*infra.Resource, error) {
	state, err := sm.backend.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("state list: %w", err)
	}
	if state == nil {
		return nil, nil
	}
	return state.Resources, nil
}

// MoveResource renames a resource in the state (does not affect the cloud resource).
func (sm *StateManager) MoveResource(ctx context.Context, from, to infra.ResourceID) error {
	state, err := sm.backend.Get(ctx)
	if err != nil {
		return fmt.Errorf("state mv: load: %w", err)
	}
	if state == nil {
		return fmt.Errorf("state mv: no state found")
	}

	found := false
	for _, r := range state.Resources {
		if r.ID == from {
			r.ID = to
			found = true
		}
		// Update any dependency references
		for i, dep := range r.DependsOn {
			if dep == from {
				r.DependsOn[i] = to
			}
		}
	}
	if !found {
		return fmt.Errorf("state mv: resource %q not found", from)
	}

	state.Serial++
	return sm.backend.Put(ctx, state)
}

// RemoveResource removes a resource from the state (does not destroy the cloud resource).
func (sm *StateManager) RemoveResource(ctx context.Context, id infra.ResourceID) error {
	state, err := sm.backend.Get(ctx)
	if err != nil {
		return fmt.Errorf("state rm: load: %w", err)
	}
	if state == nil {
		return fmt.Errorf("state rm: no state found")
	}

	found := false
	for i, r := range state.Resources {
		if r.ID == id {
			state.Resources = append(state.Resources[:i], state.Resources[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("state rm: resource %q not found", id)
	}

	state.Serial++
	return sm.backend.Put(ctx, state)
}

// Refresh updates all resource outputs with current cloud values.
func (sm *StateManager) Refresh(ctx context.Context, registry infra.ProviderRegistry) error {
	state, err := sm.backend.Get(ctx)
	if err != nil {
		return fmt.Errorf("state refresh: load: %w", err)
	}
	if state == nil {
		return nil
	}

	for i, r := range state.Resources {
		if r.State != infra.ResourceStateCreated {
			continue
		}
		provider, err := registry.Get(r.ProviderName)
		if err != nil {
			continue // skip resources with unavailable providers
		}
		updated, err := provider.Read(ctx, r)
		if err != nil {
			continue // skip unreadable resources
		}
		state.Resources[i] = updated
	}

	state.Serial++
	return sm.backend.Put(ctx, state)
}
