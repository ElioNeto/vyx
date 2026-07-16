package state

import (
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalBackend_EmptyPath(t *testing.T) {
	_, err := NewLocalBackend("")
	assert.Error(t, err)
}

func TestNewBackend_Local(t *testing.T) {
	be, err := NewBackend(infra.BackendConfig{
		Type: "local",
		Config: map[string]any{
			"path": "/tmp/test-state.json",
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, be)
}

func TestNewBackend_InvalidType(t *testing.T) {
	_, err := NewBackend(infra.BackendConfig{
		Type: "invalid",
	})
	assert.Error(t, err)
}

func TestNewBackend_Consul(t *testing.T) {
	be, err := NewBackend(infra.BackendConfig{
		Type: "consul",
		Config: map[string]any{
			"address": "http://localhost:8500",
			"path":    "vyx/test-state",
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, be)
}

func TestNewBackend_HTTP(t *testing.T) {
	be, err := NewBackend(infra.BackendConfig{
		Type: "http",
		Config: map[string]any{
			"address": "http://localhost:8080",
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, be)
}

func TestNewBackend_HTTP_NoAddress(t *testing.T) {
	_, err := NewBackend(infra.BackendConfig{
		Type:   "http",
		Config: map[string]any{},
	})
	assert.Error(t, err)
}

func TestNewBackend_S3_WithoutBuildTag(t *testing.T) {
	// Without the with_aws build tag, S3 backend returns an error
	_, err := NewBackend(infra.BackendConfig{
		Type:   "s3",
		Config: map[string]any{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "build with -tags with_aws")
}
