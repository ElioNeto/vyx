package infra

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestratorInitPlanApplyDestroy(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	orch := NewOrchestrator(
		NewPlanner(reg, backend),
		NewApplier(reg, backend),
		NewDestroyer(reg, backend),
		backend,
		reg,
	)

	// Init
	err := orch.Init(ctx)
	require.NoError(t, err)

	// Plan
	stack := createTestStack(t)
	plan, err := orch.Plan(ctx, stack)
	require.NoError(t, err)
	assert.True(t, plan.HasChanges())
	assert.Equal(t, 1, plan.Summary.ToCreate)

	// Apply
	state, err := orch.Apply(ctx, plan, stack)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Len(t, state.Resources, 1)

	// GetCurrentState
	currentState, err := orch.GetCurrentState(ctx)
	require.NoError(t, err)
	require.NotNil(t, currentState)
	assert.Equal(t, state.Serial, currentState.Serial)

	// ListProviders
	providers := orch.ListProviders()
	assert.Contains(t, providers, infra.ProviderID("mock"))

	// Destroy
	err = orch.Destroy(ctx, stack)
	require.NoError(t, err)

	// State should be gone
	currentState, err = orch.GetCurrentState(ctx)
	require.NoError(t, err)
	assert.Nil(t, currentState)
}

func TestBuildStackFromConfig(t *testing.T) {
	resources := []*infra.Resource{
		infra.NewResource("a", "mock", "mock"),
		infra.NewResource("b", "mock", "mock"),
	}

	stack := BuildStackFromConfig("test", resources)
	assert.Equal(t, "test", stack.Name)
	assert.Len(t, stack.Resources, 2)
}
