package infra

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistry(t *testing.T) {
	reg := NewProviderRegistry()
	mock := NewMockProvider("test-provider")

	err := reg.Register(mock)
	require.NoError(t, err)

	// Duplicate registration
	err = reg.Register(mock)
	assert.Error(t, err)
	assert.IsType(t, &ErrProviderAlreadyRegistered{}, err)

	// Get existing
	p, err := reg.Get("test-provider")
	require.NoError(t, err)
	assert.Equal(t, ProviderID("test-provider"), p.ID())

	// Get non-existing
	_, err = reg.Get("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, &ErrProviderNotFound{}, err)

	// List
	ids := reg.List()
	assert.Contains(t, ids, ProviderID("test-provider"))
}

func TestMockProviderCapabilities(t *testing.T) {
	mock := NewMockProvider("test")
	caps := mock.Capabilities()
	require.Len(t, caps, 1)
	assert.Equal(t, ResourceType("mock_resource"), caps[0].Type)
	assert.Contains(t, caps[0].OutputFields, "arn")
}

func TestMockProviderCreateAndRead(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider("test")

	r := NewResource("res-1", "mock_resource", "test")
	r.Properties = map[string]any{"name": "test-resource"}

	created, err := mock.Create(ctx, r)
	require.NoError(t, err)
	assert.Equal(t, ResourceStateCreated, created.State)
	assert.Contains(t, created.Outputs, "arn")

	// Read back
	read, err := mock.Read(ctx, r)
	require.NoError(t, err)
	assert.Equal(t, created.ID, read.ID)
}

func TestMockProviderPlan(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider("test")

	desired := NewResource("res-1", "mock_resource", "test")
	desired.Properties = map[string]any{"name": "test"}

	// Plan with no current resource → create
	change, err := mock.Plan(ctx, desired, nil)
	require.NoError(t, err)
	assert.Equal(t, ChangeCreate, change.ChangeType)

	// Plan with identical current resource → noop
	current := NewResource("res-1", "mock_resource", "test")
	current.State = ResourceStateCreated
	current.Properties = map[string]any{"name": "test"}
	change, err = mock.Plan(ctx, desired, current)
	require.NoError(t, err)
	assert.Equal(t, ChangeNoop, change.ChangeType)

	// Plan with different properties → update
	current.Properties["name"] = "old-name"
	change, err = mock.Plan(ctx, desired, current)
	require.NoError(t, err)
	assert.Equal(t, ChangeUpdate, change.ChangeType)
	assert.Contains(t, change.Diff, "name")
}

func TestMockProviderUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider("test")

	r := NewResource("res-1", "mock_resource", "test")
	r.Properties = map[string]any{"name": "original"}
	_, err := mock.Create(ctx, r)
	require.NoError(t, err)

	// Update
	desired := r.Clone()
	desired.Properties["name"] = "updated"
	current := r.Clone()
	current.State = ResourceStateCreated

	updated, err := mock.Update(ctx, desired, current)
	require.NoError(t, err)
	assert.Equal(t, ResourceStateCreated, updated.State)

	// Delete
	err = mock.Delete(ctx, r)
	require.NoError(t, err)

	// Read after delete should fail
	_, err = mock.Read(ctx, r)
	assert.Error(t, err)
}
