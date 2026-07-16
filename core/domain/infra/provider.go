package infra

import (
	"context"
	"encoding/json"
)

// ChangeType describes the operation needed for a resource.
type ChangeType string

const (
	ChangeCreate ChangeType = "create"
	ChangeUpdate ChangeType = "update"
	ChangeDelete ChangeType = "delete"
	ChangeNoop   ChangeType = "noop"
)

// ResourceChange represents a single planned change to a resource.
type ResourceChange struct {
	ResourceID   ResourceID          `json:"resource_id"`
	ChangeType   ChangeType          `json:"change_type"`
	ResourceType ResourceType        `json:"resource_type"`
	ProviderName ProviderID          `json:"provider"`
	Diff         map[string]DiffValue `json:"diff,omitempty"`
}

// DiffValue holds the old and new values for a changed property.
type DiffValue struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// SchemaField describes a single input field for a resource type.
type SchemaField struct {
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Description string      `json:"description"`
	Default     any         `json:"default,omitempty"`
}

// ResourceCapability describes a resource type that a provider can manage.
type ResourceCapability struct {
	Type         ResourceType            `json:"type"`
	Description  string                  `json:"description"`
	InputSchema  map[string]SchemaField  `json:"input_schema"`
	OutputFields []string                `json:"output_fields"`
}

// Provider is the contract that every cloud provider must implement.
type Provider interface {
	// ID returns the provider identifier (e.g. "aws", "gcp", "azure").
	ID() ProviderID

	// Validate checks whether the resource properties are valid.
	Validate(ctx context.Context, r *Resource) error

	// Plan determines what needs to change: create, update, delete, or noop.
	Plan(ctx context.Context, desired, current *Resource) (*ResourceChange, error)

	// Create provisions a new resource in the cloud.
	Create(ctx context.Context, r *Resource) (*Resource, error)

	// Read queries the current state of an existing resource.
	Read(ctx context.Context, r *Resource) (*Resource, error)

	// Update modifies an existing resource to match the desired state.
	Update(ctx context.Context, desired, current *Resource) (*Resource, error)

	// Delete destroys a resource in the cloud.
	Delete(ctx context.Context, r *Resource) error

	// Capabilities returns the resource types this provider can manage.
	Capabilities() []ResourceCapability
}

// ProviderRegistry manages the registration and lookup of providers.
type ProviderRegistry interface {
	// Register adds a provider to the registry.
	Register(p Provider) error

	// Get returns a provider by ID.
	Get(id ProviderID) (Provider, error)

	// List returns all registered provider IDs.
	List() []ProviderID
}

// providerRegistry is the standard implementation of ProviderRegistry.
type providerRegistry struct {
	providers map[ProviderID]Provider
}

// NewProviderRegistry creates an empty provider registry.
func NewProviderRegistry() ProviderRegistry {
	return &providerRegistry{
		providers: make(map[ProviderID]Provider),
	}
}

// Register adds a provider to the registry.
func (r *providerRegistry) Register(p Provider) error {
	id := p.ID()
	if _, exists := r.providers[id]; exists {
		return &ErrProviderAlreadyRegistered{ProviderID: id}
	}
	r.providers[id] = p
	return nil
}

// Get returns a provider by ID.
func (r *providerRegistry) Get(id ProviderID) (Provider, error) {
	p, ok := r.providers[id]
	if !ok {
		return nil, &ErrProviderNotFound{ProviderID: id}
	}
	return p, nil
}

// List returns all registered provider IDs.
func (r *providerRegistry) List() []ProviderID {
	ids := make([]ProviderID, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

// ─── MockProvider ──────────────────────────────────────────────────────────

// MockProvider is an in-memory provider for testing purposes.
// It implements Provider using in-memory storage.
type MockProvider struct {
	id           ProviderID
	resources    map[ResourceID]*Resource
	capabilities []ResourceCapability
}

// NewMockProvider creates a new MockProvider with the given ID.
func NewMockProvider(id ProviderID) *MockProvider {
	return &MockProvider{
		id:        id,
		resources: make(map[ResourceID]*Resource),
		capabilities: []ResourceCapability{
			{
				Type:        "mock_resource",
				Description: "A mock resource for testing",
				InputSchema: map[string]SchemaField{
					"name": {Type: "string", Required: true, Description: "Resource name"},
					"size": {Type: "number", Required: false, Default: 10, Description: "Size value"},
				},
				OutputFields: []string{"arn", "created_at"},
			},
		},
	}
}

func (m *MockProvider) ID() ProviderID { return m.id }

func (m *MockProvider) Validate(_ context.Context, r *Resource) error {
	if r.Properties == nil {
		return &ErrValidation{ResourceID: r.ID, Message: "properties is nil"}
	}
	return nil
}

func (m *MockProvider) Plan(_ context.Context, desired, current *Resource) (*ResourceChange, error) {
	if current == nil || current.State == ResourceStateDeleted || current.State == ResourceStatePending {
		return &ResourceChange{
			ResourceID:   desired.ID,
			ChangeType:   ChangeCreate,
			ResourceType: desired.Type,
			ProviderName: m.id,
		}, nil
	}

	// Compare properties, normalizing types after JSON round-trip.
	diff := make(map[string]DiffValue)
	for k, newVal := range desired.Properties {
		oldVal, ok := current.Properties[k]
		if !ok || !valuesEqual(oldVal, newVal) {
			diff[k] = DiffValue{Old: oldVal, New: newVal}
		}
	}
	for k, oldVal := range current.Properties {
		if _, ok := desired.Properties[k]; !ok {
			diff[k] = DiffValue{Old: oldVal, New: nil}
		}
	}

	if len(diff) == 0 {
		return &ResourceChange{
			ResourceID:   desired.ID,
			ChangeType:   ChangeNoop,
			ResourceType: desired.Type,
			ProviderName: m.id,
		}, nil
	}

	return &ResourceChange{
		ResourceID:   desired.ID,
		ChangeType:   ChangeUpdate,
		ResourceType: desired.Type,
		ProviderName: m.id,
		Diff:         diff,
	}, nil
}

// valuesEqual compares two values, handling JSON round-trip type changes.
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Normalize numeric types: compare as float64 after JSON round-trip.
	af, aOk := toFloat64(a)
	bf, bOk := toFloat64(b)
	if aOk && bOk {
		return af == bf
	}
	return a == b
}

func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case int16:
		return float64(val), true
	case int8:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint8:
		return float64(val), true
	case json.Number:
		f, _ := val.Float64()
		return f, true
	default:
		return 0, false
	}
}

func (m *MockProvider) Create(_ context.Context, r *Resource) (*Resource, error) {
	created := r.Clone()
	created.State = ResourceStateCreated
	created.Outputs = map[string]string{
		"arn":        "arn:mock:" + string(r.ID),
		"created_at": Now().String(),
	}
	m.resources[r.ID] = created
	return created, nil
}

func (m *MockProvider) Read(_ context.Context, r *Resource) (*Resource, error) {
	// First try direct ID lookup.
	if existing, ok := m.resources[r.ID]; ok {
		return existing.Clone(), nil
	}
	// Then try looking up by "id" property (used by Importer).
	if cloudID, ok := r.Properties["id"].(string); ok {
		if existing, ok := m.resources[ResourceID(cloudID)]; ok {
			return existing.Clone(), nil
		}
	}
	return nil, &ErrResourceNotFound{ResourceID: r.ID}
}

func (m *MockProvider) Update(_ context.Context, desired, current *Resource) (*Resource, error) {
	updated := desired.Clone()
	updated.State = ResourceStateCreated
	m.resources[desired.ID] = updated
	return updated, nil
}

func (m *MockProvider) Delete(_ context.Context, r *Resource) error {
	delete(m.resources, r.ID)
	return nil
}

func (m *MockProvider) Capabilities() []ResourceCapability {
	return m.capabilities
}
