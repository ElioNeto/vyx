package infra

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// Orchestrator is the unified facade for infrastructure operations.
// It coordinates planning, applying, and destroying resources.
type Orchestrator struct {
	planner   *Planner
	applier   *Applier
	destroyer *Destroyer
	backend   infra.Backend
	registry  infra.ProviderRegistry
}

// NewOrchestrator creates a new Orchestrator.
func NewOrchestrator(
	planner *Planner,
	applier *Applier,
	destroyer *Destroyer,
	backend infra.Backend,
	registry infra.ProviderRegistry,
) *Orchestrator {
	return &Orchestrator{
		planner:   planner,
		applier:   applier,
		destroyer: destroyer,
		backend:   backend,
		registry:  registry,
	}
}

// Init initializes the state backend.
func (o *Orchestrator) Init(ctx context.Context) error {
	return o.backend.Init(ctx)
}

// Plan computes and returns the plan for the given stack.
func (o *Orchestrator) Plan(ctx context.Context, stack *infra.Stack) (*infra.PlanResult, error) {
	return o.planner.Plan(ctx, stack)
}

// Apply executes the plan and returns the new state.
func (o *Orchestrator) Apply(ctx context.Context, plan *infra.PlanResult, stack *infra.Stack) (*infra.State, error) {
	return o.applier.Apply(ctx, plan, stack)
}

// Destroy removes all resources in the stack.
func (o *Orchestrator) Destroy(ctx context.Context, stack *infra.Stack) error {
	return o.destroyer.Destroy(ctx, stack)
}

// GetCurrentState returns the current state from the backend.
func (o *Orchestrator) GetCurrentState(ctx context.Context) (*infra.State, error) {
	return o.backend.Get(ctx)
}

// RegisterProvider adds a provider to the registry.
func (o *Orchestrator) RegisterProvider(p infra.Provider) error {
	return o.registry.Register(p)
}

// Backend returns the state backend used by this orchestrator.
func (o *Orchestrator) Backend() infra.Backend {
	return o.backend
}

// Registry returns the provider registry used by this orchestrator.
func (o *Orchestrator) Registry() infra.ProviderRegistry {
	return o.registry
}

// ListProviders returns all registered provider IDs.
func (o *Orchestrator) ListProviders() []infra.ProviderID {
	return o.registry.List()
}

// BuildStackFromConfig creates a Stack from resource configurations.
// This is a convenience method that takes resource definitions (from annotations or YAML)
// and builds a Stack ready for planning.
func BuildStackFromConfig(name string, resources []*infra.Resource) *infra.Stack {
	stack := infra.NewStack(name)
	for _, r := range resources {
		stack.AddResource(r)
	}
	return stack
}

// AcquireLock acquires a lock for the given operation.
func (o *Orchestrator) AcquireLock(ctx context.Context, operation string) error {
	info := infra.LockInfo{
		ID:        fmt.Sprintf("vyx-infra-%s", operation),
		Operation: operation,
		Who:       "vyx-cli",
	}
	return o.backend.Lock(ctx, info)
}

// ReleaseLock releases a previously acquired lock.
func (o *Orchestrator) ReleaseLock(ctx context.Context) error {
	info := infra.LockInfo{
		ID:        "vyx-infra-lock",
		Operation: "release",
		Who:       "vyx-cli",
	}
	return o.backend.Unlock(ctx, info)
}
