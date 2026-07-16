package infra

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infrastructurestate "github.com/ElioNeto/vyx/core/infrastructure/infra/state"
)

func TestPlannerCreate(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	planner := NewPlanner(reg, backend)

	// Create a stack with one resource.
	stack := infra.NewStack("test")
	r := infra.NewResource("bucket-1", "mock_resource", "mock")
	r.Properties = map[string]any{"name": "test-bucket"}
	stack.AddResource(r)

	plan, err := planner.Plan(ctx, stack)
	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.Equal(t, 1, plan.Summary.ToCreate)
	assert.Equal(t, 0, plan.Summary.ToUpdate)
	assert.Equal(t, 0, plan.Summary.ToDelete)
	assert.True(t, plan.HasChanges())
}

func TestPlannerNoChanges(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	// First, create and apply to have state.
	stack := createTestStack(t)

	applier := NewApplier(reg, backend)
	planner := NewPlanner(reg, backend)

	plan, err := planner.Plan(ctx, stack)
	require.NoError(t, err)
	require.NotNil(t, plan)

	// Apply the plan first.
	_, err = applier.Apply(ctx, plan, stack)
	require.NoError(t, err)

	// Now plan again — should be no changes.
	plan, err = planner.Plan(ctx, stack)
	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.Equal(t, 0, plan.Summary.ToCreate)
	assert.Equal(t, 0, plan.Summary.ToUpdate)
	assert.Equal(t, 0, plan.Summary.ToDelete)
	assert.False(t, plan.HasChanges())
}

func TestPlannerDelete(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	applier := NewApplier(reg, backend)
	planner := NewPlanner(reg, backend)

	// Create and apply a stack.
	fullStack := createTestStack(t)
	plan, err := planner.Plan(ctx, fullStack)
	require.NoError(t, err)
	_, err = applier.Apply(ctx, plan, fullStack)
	require.NoError(t, err)

	// Now plan with an empty stack (removes all resources).
	emptyStack := infra.NewStack("test")
	plan, err = planner.Plan(ctx, emptyStack)
	require.NoError(t, err)

	assert.Equal(t, 1, plan.Summary.ToDelete)
	assert.True(t, plan.HasChanges())
}

// ─── Test Helpers ────────────────────────────────────────────────────────

func setupTestInfra(t *testing.T) (infra.ProviderRegistry, infra.Backend, func()) {
	t.Helper()

	reg := infra.NewProviderRegistry()
	mock := infra.NewMockProvider("mock")
	err := reg.Register(mock)
	require.NoError(t, err)

	// Create a temporary directory for local state backend.
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.tfstate")

	backend, err := infrastructurestate.NewLocalBackend(statePath)
	require.NoError(t, err)

	cleanup := func() {
		os.Remove(statePath)
	}

	return reg, backend, cleanup
}

func createTestStack(t *testing.T) *infra.Stack {
	t.Helper()
	stack := infra.NewStack("test")
	r := infra.NewResource("bucket-1", "mock_resource", "mock")
	r.Properties = map[string]any{"name": "test-bucket", "size": 10}
	stack.AddResource(r)
	return stack
}
