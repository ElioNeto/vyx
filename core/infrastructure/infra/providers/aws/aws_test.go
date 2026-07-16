package aws

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderInterface(t *testing.T) {
	p, err := New(map[string]any{
		"region": "us-east-1",
	})
	require.NoError(t, err)

	// Provider ID
	assert.Equal(t, infra.ProviderID("aws"), p.ID())

	// Capabilities
	caps := p.Capabilities()
	assert.Len(t, caps, 11) // 11 resource types

	// Check required resource types exist
	capTypes := make(map[infra.ResourceType]bool)
	for _, c := range caps {
		capTypes[c.Type] = true
	}
	assert.True(t, capTypes["aws_s3_bucket"])
	assert.True(t, capTypes["aws_instance"])
	assert.True(t, capTypes["aws_db_instance"])
	assert.True(t, capTypes["aws_iam_role"])
	assert.True(t, capTypes["aws_lambda_function"])
	assert.True(t, capTypes["aws_vpc"])
	assert.True(t, capTypes["aws_sqs_queue"])
	assert.True(t, capTypes["aws_sns_topic"])
	assert.True(t, capTypes["aws_route53_zone"])
	assert.True(t, capTypes["aws_elasticache_cluster"])
	assert.True(t, capTypes["aws_api_gateway_rest_api"])
}

func TestProviderValidate(t *testing.T) {
	p, err := New(map[string]any{"region": "us-east-1"})
	require.NoError(t, err)

	ctx := context.Background()

	// Valid: properties present
	r := infra.NewResource("test", "aws_s3_bucket", "aws")
	r.Properties = map[string]any{"bucket": "test-bucket"}
	err = p.Validate(ctx, r)
	assert.NoError(t, err)

	// Invalid: nil properties
	r2 := infra.NewResource("bad", "aws_s3_bucket", "aws")
	err = p.Validate(ctx, r2)
	assert.Error(t, err)
	assert.IsType(t, &infra.ErrValidation{}, err)
}

func TestProviderPlanCreate(t *testing.T) {
	p, err := New(map[string]any{"region": "us-east-1"})
	require.NoError(t, err)

	ctx := context.Background()

	desired := infra.NewResource("bucket", "aws_s3_bucket", "aws")
	desired.Properties = map[string]any{"bucket": "my-test-bucket"}

	// Plan with no current resource = create
	change, err := p.Plan(ctx, desired, nil)
	require.NoError(t, err)
	assert.Equal(t, infra.ChangeCreate, change.ChangeType)
	assert.Equal(t, infra.ResourceID("bucket"), change.ResourceID)
}

func TestProviderPlanNoop(t *testing.T) {
	p, err := New(map[string]any{"region": "us-east-1"})
	require.NoError(t, err)

	ctx := context.Background()

	desired := infra.NewResource("bucket", "aws_s3_bucket", "aws")
	desired.Properties = map[string]any{"bucket": "test", "acl": "private"}

	current := infra.NewResource("bucket", "aws_s3_bucket", "aws")
	current.State = infra.ResourceStateCreated
	current.Properties = map[string]any{"bucket": "test", "acl": "private"}

	change, err := p.Plan(ctx, desired, current)
	require.NoError(t, err)
	assert.Equal(t, infra.ChangeNoop, change.ChangeType)
}

func TestProviderPlanUpdate(t *testing.T) {
	p, err := New(map[string]any{"region": "us-east-1"})
	require.NoError(t, err)

	ctx := context.Background()

	desired := infra.NewResource("bucket", "aws_s3_bucket", "aws")
	desired.Properties = map[string]any{"bucket": "test", "versioning": true}

	current := infra.NewResource("bucket", "aws_s3_bucket", "aws")
	current.State = infra.ResourceStateCreated
	current.Properties = map[string]any{"bucket": "test", "versioning": false}

	change, err := p.Plan(ctx, desired, current)
	require.NoError(t, err)
	assert.Equal(t, infra.ChangeUpdate, change.ChangeType)
	assert.Contains(t, change.Diff, "versioning")
}

func TestProviderPlanUnknownType(t *testing.T) {
	p, err := New(map[string]any{"region": "us-east-1"})
	require.NoError(t, err)

	ctx := context.Background()
	desired := infra.NewResource("x", "unknown_type", "aws")

	_, err = p.Plan(ctx, desired, nil)
	assert.Error(t, err)
	assert.IsType(t, &infra.ErrValidation{}, err)
}

func TestProviderResourceTypesCoverage(t *testing.T) {
	p, err := New(map[string]any{"region": "us-east-1"})
	require.NoError(t, err)

	// Verify each resource type has a plan function
	resourceTypes := []infra.ResourceType{
		"aws_s3_bucket", "aws_instance", "aws_db_instance", "aws_iam_role",
		"aws_lambda_function", "aws_vpc", "aws_sqs_queue", "aws_sns_topic",
		"aws_route53_zone", "aws_elasticache_cluster", "aws_api_gateway_rest_api",
	}

	ctx := context.Background()
	for _, rt := range resourceTypes {
		desired := infra.NewResource("test-"+string(rt), rt, "aws")
		desired.Properties = map[string]any{"name": "test"}

		change, err := p.Plan(ctx, desired, nil)
		require.NoError(t, err, "Plan failed for type %s", rt)
		require.Equal(t, infra.ChangeCreate, change.ChangeType, "Unexpected change for %s", rt)
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		a, b any
		want bool
	}{
		{10, 10.0, true},
		{10, 20, false},
		{10, "10", false},
		{"hello", "hello", true},
		{nil, nil, true},
		{nil, "x", false},
		{int64(100), float64(100), true},
		{true, true, true},
		{false, true, false},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := valuesEqual(tc.a, tc.b)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetStringProp(t *testing.T) {
	props := map[string]any{"name": "test", "count": 42}

	assert.Equal(t, "test", getStringProp(props, "name", "default"))
	assert.Equal(t, "default", getStringProp(props, "missing", "default"))
	assert.Equal(t, "42", getStringProp(props, "count", "default"))
}

func TestGetBoolProp(t *testing.T) {
	props := map[string]any{"enabled": true, "flag": "true"}

	assert.True(t, getBoolProp(props, "enabled", false))
	assert.True(t, getBoolProp(props, "flag", false))
	assert.False(t, getBoolProp(props, "missing", false))
}

func TestGetIntProp(t *testing.T) {
	props := map[string]any{"size": 100, "count": 42.5}

	assert.Equal(t, 100, getIntProp(props, "size", 0))
	assert.Equal(t, 42, getIntProp(props, "count", 0))
	assert.Equal(t, 10, getIntProp(props, "missing", 10))
}
