package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewResource(t *testing.T) {
	r := NewResource("my-bucket", "aws_s3_bucket", "aws")

	assert.Equal(t, ResourceID("my-bucket"), r.ID)
	assert.Equal(t, ResourceType("aws_s3_bucket"), r.Type)
	assert.Equal(t, ProviderID("aws"), r.ProviderName)
	assert.Equal(t, ResourceStatePending, r.State)
	assert.Empty(t, r.Properties)
	assert.Empty(t, r.Outputs)
	assert.Empty(t, r.DependsOn)
}

func TestResourceIsManaged(t *testing.T) {
	tests := []struct {
		name   string
		state  ResourceState
		want   bool
	}{
		{"created is managed", ResourceStateCreated, true},
		{"updating is managed", ResourceStateUpdating, true},
		{"pending is not managed", ResourceStatePending, false},
		{"failed is not managed", ResourceStateFailed, false},
		{"deleted is not managed", ResourceStateDeleted, false},
		{"deleting is not managed", ResourceStateDeleting, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := NewResource("test", "mock", "mock")
			r.State = tc.state
			assert.Equal(t, tc.want, r.IsManaged())
		})
	}
}

func TestResourceNeedsCreation(t *testing.T) {
	r := NewResource("test", "mock", "mock")
	assert.True(t, r.NeedsCreation())

	r.State = ResourceStateFailed
	assert.True(t, r.NeedsCreation())

	r.State = ResourceStateCreated
	assert.False(t, r.NeedsCreation())
}

func TestResourceClone(t *testing.T) {
	original := NewResource("test", "mock", "mock")
	original.Properties["key"] = "value"
	original.Outputs["out"] = "val"
	original.DependsOn = []ResourceID{"dep1"}
	original.Tags["env"] = "prod"

	clone := original.Clone()

	// Values match
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.Properties["key"], clone.Properties["key"])
	assert.Equal(t, original.Outputs["out"], clone.Outputs["out"])
	assert.Equal(t, original.DependsOn, clone.DependsOn)
	assert.Equal(t, original.Tags["env"], clone.Tags["env"])

	// Mutations don't affect the other
	original.Properties["key"] = "changed"
	assert.Equal(t, "value", clone.Properties["key"])

	clone.Properties["new"] = "newval"
	assert.NotContains(t, original.Properties, "new")
}

func TestResourceOutputsIsZero(t *testing.T) {
	var ro ResourceOutputs
	assert.True(t, ro.IsZero())

	ro = ResourceOutputs{"res1": {"key": "val"}}
	assert.False(t, ro.IsZero())
}
