package infra

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateManagerList(t *testing.T) {
	ctx := context.Background()
	_, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	sm := NewStateManager(backend)

	// Empty state
	resources, err := sm.ListResources(ctx)
	require.NoError(t, err)
	assert.Nil(t, resources)

	// Add resource
	state := infra.NewState("test")
	state.Resources = append(state.Resources, infra.NewResource("r1", "mock", "mock"))
	require.NoError(t, backend.Put(ctx, state))

	resources, err = sm.ListResources(ctx)
	require.NoError(t, err)
	assert.Len(t, resources, 1)
}

func TestStateManagerMove(t *testing.T) {
	ctx := context.Background()
	_, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	sm := NewStateManager(backend)

	state := infra.NewState("test")
	r1 := infra.NewResource("old-name", "mock", "mock")
	r2 := infra.NewResource("other", "mock", "mock")
	r2.DependsOn = []infra.ResourceID{"old-name"}
	state.Resources = append(state.Resources, r1, r2)
	require.NoError(t, backend.Put(ctx, state))

	// Move
	err := sm.MoveResource(ctx, "old-name", "new-name")
	require.NoError(t, err)

	// Verify
	resources, err := sm.ListResources(ctx)
	require.NoError(t, err)
	assert.Equal(t, infra.ResourceID("new-name"), resources[0].ID)
	assert.Equal(t, []infra.ResourceID{"new-name"}, resources[1].DependsOn)
}

func TestStateManagerRemove(t *testing.T) {
	ctx := context.Background()
	_, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	sm := NewStateManager(backend)

	state := infra.NewState("test")
	state.Resources = append(state.Resources,
		infra.NewResource("r1", "mock", "mock"),
		infra.NewResource("r2", "mock", "mock"),
	)
	require.NoError(t, backend.Put(ctx, state))

	err := sm.RemoveResource(ctx, "r1")
	require.NoError(t, err)

	resources, err := sm.ListResources(ctx)
	require.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, infra.ResourceID("r2"), resources[0].ID)
}

func TestStateManagerMoveNotFound(t *testing.T) {
	ctx := context.Background()
	_, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	sm := NewStateManager(backend)
	err := sm.MoveResource(ctx, "nonexistent", "new")
	assert.Error(t, err)
}

func TestStateManagerRemoveNotFound(t *testing.T) {
	ctx := context.Background()
	_, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	sm := NewStateManager(backend)
	err := sm.RemoveResource(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestImporter(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	im := NewImporter(reg, backend)

	// First create a resource in the mock provider
	mockProvider, _ := reg.Get("mock")
	created, err := mockProvider.Create(ctx, infra.NewResource("existing-bucket", "mock_resource", "mock"))
	require.NoError(t, err)
	require.NotNil(t, created)

	// Now import it
	result, err := im.Import(ctx, "mock_resource", "my-bucket", "existing-bucket")
	require.NoError(t, err)
	assert.True(t, result.Found)
	assert.Equal(t, infra.ResourceID("my-bucket"), result.ResourceID)

	// Verify it's in the state
	state, err := backend.Get(ctx)
	require.NoError(t, err)
	assert.Len(t, state.Resources, 1)
	assert.Equal(t, infra.ResourceID("my-bucket"), state.Resources[0].ID)
}

func TestImporterDuplicate(t *testing.T) {
	ctx := context.Background()
	reg, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	im := NewImporter(reg, backend)

	// Add resource to state first
	state := infra.NewState("test")
	state.Resources = append(state.Resources, infra.NewResource("my-bucket", "mock_resource", "mock"))
	require.NoError(t, backend.Put(ctx, state))

	// Try to import same ID
	mockProvider, _ := reg.Get("mock")
	mockProvider.Create(ctx, infra.NewResource("existing", "mock_resource", "mock"))

	_, err := im.Import(ctx, "mock_resource", "my-bucket", "existing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestImporterNoProvider(t *testing.T) {
	ctx := context.Background()
	_, backend, cleanup := setupTestInfra(t)
	defer cleanup()

	reg := infra.NewProviderRegistry() // empty registry
	im := NewImporter(reg, backend)

	_, err := im.Import(ctx, "unknown_type", "test", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no provider found")
}
