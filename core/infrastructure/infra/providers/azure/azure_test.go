package azure

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderID(t *testing.T) {
	p := &Provider{id: "azure"}
	assert.Equal(t, infra.ProviderID("azure"), p.ID())
}

func TestProviderCapabilities(t *testing.T) {
	p := &Provider{}
	caps := p.Capabilities()
	assert.Len(t, caps, 3)

	capTypes := make(map[infra.ResourceType]bool)
	for _, c := range caps {
		capTypes[c.Type] = true
	}
	assert.True(t, capTypes["azure_storage_account"])
	assert.True(t, capTypes["azure_virtual_machine"])
	assert.True(t, capTypes["azure_sql_database"])
}

func TestProviderValidate(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()

	r := infra.NewResource("test", "azure_storage_account", "azure")
	r.Properties = map[string]any{"name": "mystorage"}
	err := p.Validate(ctx, r)
	assert.NoError(t, err)

	r2 := infra.NewResource("bad", "azure_storage_account", "azure")
	err = p.Validate(ctx, r2)
	assert.Error(t, err)
}

func TestProviderPlanCreate(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()

	desired := infra.NewResource("sa", "azure_storage_account", "azure")
	desired.Properties = map[string]any{"name": "test"}

	change, err := p.Plan(ctx, desired, nil)
	require.NoError(t, err)
	assert.Equal(t, infra.ChangeCreate, change.ChangeType)
}

func TestProviderPlanNoop(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()

	desired := infra.NewResource("sa", "azure_storage_account", "azure")
	desired.Properties = map[string]any{"name": "test", "sku": "Standard_LRS"}

	current := infra.NewResource("sa", "azure_storage_account", "azure")
	current.State = infra.ResourceStateCreated
	current.Properties = map[string]any{"name": "test", "sku": "Standard_LRS"}

	change, err := p.Plan(ctx, desired, current)
	require.NoError(t, err)
	assert.Equal(t, infra.ChangeNoop, change.ChangeType)
}

func TestProviderCreate(t *testing.T) {
	p := &Provider{resourceGroup: "test-rg", subscription: "sub-123"}
	ctx := context.Background()

	r := infra.NewResource("storage1", "azure_storage_account", "azure")
	r.Properties = map[string]any{"name": "storage1"}

	created, err := p.Create(ctx, r)
	require.NoError(t, err)
	assert.Equal(t, infra.ResourceStateCreated, created.State)
	assert.Contains(t, created.Outputs, "id")
	assert.Contains(t, created.Outputs, "name")
}

func TestHelpersGetStringProp(t *testing.T) {
	props := map[string]any{"name": "test", "count": 42}
	assert.Equal(t, "test", getStringProp(props, "name", ""))
	assert.Equal(t, "default", getStringProp(props, "missing", "default"))
}

func TestHelpersValuesEqual(t *testing.T) {
	assert.True(t, valuesEqual(10, 10.0))
	assert.True(t, valuesEqual("hello", "hello"))
	assert.False(t, valuesEqual(10, "10"))
	assert.True(t, valuesEqual(nil, nil))
	assert.False(t, valuesEqual(nil, "x"))
}
