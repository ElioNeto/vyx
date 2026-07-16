package infra

import (
	"context"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infrastructurestate "github.com/ElioNeto/vyx/core/infrastructure/infra/state"
)

func TestOutputCollectorAllOutputs(t *testing.T) {
	ctx := context.Background()
	backend := setupOutputBackend(t)
	defer backend.Cleanup()

	state := infra.NewState("test")
	r := infra.NewResource("bucket", "aws_s3_bucket", "aws")
	r.Outputs = map[string]string{
		"arn":  "arn:aws:s3:::my-bucket",
		"host": "my-bucket.s3.amazonaws.com",
	}
	state.Resources = append(state.Resources, r)
	require.NoError(t, backend.Put(ctx, state))

	collector := NewOutputCollector(backend)
	entries, err := collector.AllOutputs(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, infra.ResourceID("bucket"), entries[0].ResourceID)
	assert.Equal(t, "arn", entries[0].Name)
}

func TestOutputCollectorGetOutput(t *testing.T) {
	ctx := context.Background()
	backend := setupOutputBackend(t)
	defer backend.Cleanup()

	state := infra.NewState("test")
	r := infra.NewResource("bucket", "aws_s3_bucket", "aws")
	r.Outputs = map[string]string{"arn": "arn:aws:s3:::my-bucket", "host": "example.com"}
	state.Resources = append(state.Resources, r)
	require.NoError(t, backend.Put(ctx, state))

	collector := NewOutputCollector(backend)

	val, err := collector.GetOutput(ctx, "bucket.arn")
	require.NoError(t, err)
	assert.Equal(t, "arn:aws:s3:::my-bucket", val)

	val, err = collector.GetOutput(ctx, "bucket.host")
	require.NoError(t, err)
	assert.Equal(t, "example.com", val)

	_, err = collector.GetOutput(ctx, "bucket.nonexistent")
	assert.Error(t, err)

	_, err = collector.GetOutput(ctx, "nonexistent.arn")
	assert.Error(t, err)

	_, err = collector.GetOutput(ctx, "invalidref")
	assert.Error(t, err)
}

func TestOutputCollectorEmptyState(t *testing.T) {
	ctx := context.Background()
	backend := setupOutputBackend(t)
	defer backend.Cleanup()

	collector := NewOutputCollector(backend)

	entries, err := collector.AllOutputs(ctx)
	require.NoError(t, err)
	assert.Empty(t, entries)

	outputs, err := collector.OutputsMap(ctx)
	require.NoError(t, err)
	assert.Nil(t, outputs)
}

type testBackend struct {
	infra.Backend
	cleanup func()
}

func (b *testBackend) Cleanup() { b.cleanup() }

func setupOutputBackend(t *testing.T) *testBackend {
	t.Helper()
	tmpDir := t.TempDir()
	backend, err := infrastructurestate.NewLocalBackend(tmpDir + "/state.tfstate")
	require.NoError(t, err)
	require.NoError(t, backend.Init(context.Background()))

	return &testBackend{
		Backend: backend,
		cleanup: func() {},
	}
}
