package infra

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplierCreate(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	applier := NewApplier(reg, backend)
	planner := NewPlanner(reg, backend)

	stack := createTestStack(t)

	plan, err := planner.Plan(ctx, stack)
	require.NoError(t, err)

	state, err := applier.Apply(ctx, plan, stack)
	require.NoError(t, err)
	require.NotNil(t, state)

	assert.Len(t, state.Resources, 1)
	assert.Equal(t, uint64(1), state.Serial) // first apply uses state serial 1
}

func TestApplierUpdate(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	applier := NewApplier(reg, backend)
	planner := NewPlanner(reg, backend)

	// Create initial resource.
	stack := createTestStack(t)
	plan, err := planner.Plan(ctx, stack)
	require.NoError(t, err)
	_, err = applier.Apply(ctx, plan, stack)
	require.NoError(t, err)

	// Now modify the resource.
	stack.Resources[0].Properties["size"] = 20
	plan, err = planner.Plan(ctx, stack)
	require.NoError(t, err)
	assert.Equal(t, 1, plan.Summary.ToUpdate)

	state, err := applier.Apply(ctx, plan, stack)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Len(t, state.Resources, 1)
}

func TestApplierDelete(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	applier := NewApplier(reg, backend)
	planner := NewPlanner(reg, backend)

	// Create resource.
	stack := createTestStack(t)
	plan, err := planner.Plan(ctx, stack)
	require.NoError(t, err)
	_, err = applier.Apply(ctx, plan, stack)
	require.NoError(t, err)

	// Now delete — empty stack.
	emptyStack := infra.NewStack("test")
	plan, err = planner.Plan(ctx, emptyStack)
	require.NoError(t, err)
	assert.Equal(t, 1, plan.Summary.ToDelete)

	state, err := applier.Apply(ctx, plan, emptyStack)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Empty(t, state.Resources)
}

func TestApplierNoop(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	applier := NewApplier(reg, backend)
	planner := NewPlanner(reg, backend)

	// Apply once.
	stack := createTestStack(t)
	plan, err := planner.Plan(ctx, stack)
	require.NoError(t, err)
	_, err = applier.Apply(ctx, plan, stack)
	require.NoError(t, err)

	// Plan with same state — should be noop.
	plan, err = planner.Plan(ctx, stack)
	require.NoError(t, err)
	assert.False(t, plan.HasChanges())

	// Apply noop plan.
	state, err := applier.Apply(ctx, plan, stack)
	require.NoError(t, err)
	assert.Nil(t, state) // noop returns nil state
}
