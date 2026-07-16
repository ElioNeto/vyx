package infra

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestroyer(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	destroyer := NewDestroyer(reg, backend)
	applier := NewApplier(reg, backend)
	planner := NewPlanner(reg, backend)

	// Create and apply a resource.
	stack := createTestStack(t)
	plan, err := planner.Plan(ctx, stack)
	require.NoError(t, err)
	_, err = applier.Apply(ctx, plan, stack)
	require.NoError(t, err)

	// Verify state exists.
	state, err := backend.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Len(t, state.Resources, 1)

	// Destroy.
	err = destroyer.Destroy(ctx, stack)
	require.NoError(t, err)

	// Verify state is gone.
	state, err = backend.Get(ctx)
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestDestroyerEmptyStack(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	destroyer := NewDestroyer(reg, backend)

	// Destroy empty stack should succeed.
	stack := infra.NewStack("empty")
	err := destroyer.Destroy(ctx, stack)
	require.NoError(t, err)
}
