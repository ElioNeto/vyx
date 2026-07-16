//go:build integration

package infra

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LocalStack endpoint for integration tests.
const localStackEndpoint = "http://localhost:4566"

func skipIfNoLocalStack(t *testing.T) {
	t.Helper()
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("skipping integration test")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(localStackEndpoint + "/_localstack/health")
	if err != nil {
		t.Skipf("LocalStack not available at %s: %v", localStackEndpoint, err)
	}
	resp.Body.Close()
}

func TestIntegration_MockProviderPlanApplyCycle(t *testing.T) {
	ctx := context.Background()

	reg := infra.NewProviderRegistry()
	mock := infra.NewMockProvider("mock")
	require.NoError(t, reg.Register(mock))

	r := infra.NewResource("bucket-1", "mock_resource", "mock")
	r.Properties = map[string]any{"name": "test-bucket"}

	// Plan: should be create
	p := infra.NewProvider(reg)
	if pp, ok := p.(interface{ Plan(context.Context, *infra.Resource, *infra.Resource) (*infra.ResourceChange, error) }); ok {
		change, err := pp.Plan(ctx, r, nil)
		require.NoError(t, err)
		assert.Equal(t, infra.ChangeCreate, change.ChangeType)
	}

	// Create
	created, err := mock.Create(ctx, r)
	require.NoError(t, err)
	assert.Equal(t, infra.ResourceStateCreated, created.State)
	assert.Contains(t, created.Outputs, "arn")

	// Read back
	read, err := mock.Read(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, created.ID, read.ID)

	// Plan against existing: should be noop
	change, err := mock.Plan(ctx, r, read)
	require.NoError(t, err)
	assert.Equal(t, infra.ChangeNoop, change.ChangeType)

	// Delete
	err = mock.Delete(ctx, r)
	require.NoError(t, err)
}

func TestIntegration_GraphGeneration(t *testing.T) {
	stack := infra.NewStack("integration-test")

	r1 := infra.NewResource("vpc", "mock_resource", "mock")
	r1.Properties = map[string]any{"cidr_block": "10.0.0.0/16"}

	r2 := infra.NewResource("subnet", "mock_resource", "mock")
	r2.Properties = map[string]any{"cidr_block": "10.0.1.0/24"}
	r2.DependsOn = []infra.ResourceID{"vpc"}

	r3 := infra.NewResource("instance", "mock_resource", "mock")
	r3.Properties = map[string]any{"ami": "ami-123", "instance_type": "t3.micro"}
	r3.DependsOn = []infra.ResourceID{"subnet"}

	stack.AddResource(r1)
	stack.AddResource(r2)
	stack.AddResource(r3)

	sorted, err := stack.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, sorted, 3)

	idx := map[infra.ResourceID]int{}
	for i, r := range sorted {
		idx[r.ID] = i
	}
	assert.Less(t, idx["vpc"], idx["subnet"])
	assert.Less(t, idx["subnet"], idx["instance"])
}

func TestIntegration_TopologicalSortReverse(t *testing.T) {
	stack := infra.NewStack("destroy-test")

	r1 := infra.NewResource("app", "mock_resource", "mock")
	r2 := infra.NewResource("db", "mock_resource", "mock")
	r1.DependsOn = []infra.ResourceID{"db"}

	stack.AddResource(r1)
	stack.AddResource(r2)

	sorted, err := stack.TopologicalSortReverse()
	require.NoError(t, err)
	require.Len(t, sorted, 2)
	assert.Equal(t, infra.ResourceID("app"), sorted[0].ID)
	assert.Equal(t, infra.ResourceID("db"), sorted[1].ID)
}

func TestIntegration_TerraformExport(t *testing.T) {
	stack := infra.NewStack("export-test")

	r := infra.NewResource("bucket", "aws_s3_bucket", "aws")
	r.Properties = map[string]any{"bucket": "test-bucket", "acl": "private"}
	stack.AddResource(r)

	// Just validate the stack is well-formed
	assert.Equal(t, 1, stack.ResourceCount())
	assert.Equal(t, infra.ResourceID("bucket"), stack.FindResource("bucket").ID)
}
