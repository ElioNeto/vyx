package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultInfraConfig(t *testing.T) {
	cfg := DefaultInfraConfig()

	assert.Equal(t, "local", cfg.Backend.Type)
	assert.NotNil(t, cfg.Backend.Config)
	assert.Equal(t, ".vyx/infra.tfstate", cfg.Backend.Config["path"])
	assert.Empty(t, cfg.Providers)
	assert.Empty(t, cfg.Stacks)
	assert.Empty(t, cfg.DefaultTags)
}

func TestInfraConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  InfraConfig
		wantErr bool
	}{
		{
			name:    "valid local backend",
			config:  InfraConfig{Backend: BackendConfig{Type: "local"}},
			wantErr: false,
		},
		{
			name:    "valid s3 backend",
			config:  InfraConfig{Backend: BackendConfig{Type: "s3"}},
			wantErr: false,
		},
		{
			name:    "invalid backend type",
			config:  InfraConfig{Backend: BackendConfig{Type: "mysql"}},
			wantErr: true,
		},
		{
			name:    "empty backend type",
			config:  InfraConfig{},
			wantErr: true,
		},
		{
			name: "with providers",
			config: InfraConfig{
				Backend:   BackendConfig{Type: "local"},
				Providers: []ProviderConfig{{Name: "aws"}},
			},
			wantErr: false,
		},
		{
			name: "empty provider name",
			config: InfraConfig{
				Backend:   BackendConfig{Type: "local"},
				Providers: []ProviderConfig{{Name: ""}},
			},
			wantErr: true,
		},
		{
			name: "with stacks",
			config: InfraConfig{
				Backend: BackendConfig{Type: "local"},
				Stacks:  []StackConfig{{Name: "default"}},
			},
			wantErr: false,
		},
		{
			name: "empty stack name",
			config: InfraConfig{
				Backend: BackendConfig{Type: "local"},
				Stacks:  []StackConfig{{Name: ""}},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProviderConfigValidation(t *testing.T) {
	cfg := InfraConfig{
		Backend: BackendConfig{Type: "local"},
		Providers: []ProviderConfig{
			{Name: "aws"},
			{Name: ""}, // invalid
			{Name: "gcp"},
		},
	}
	err := cfg.Validate()
	assert.Error(t, err)
}
