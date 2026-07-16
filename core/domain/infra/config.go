package infra

import (
	"errors"
	"fmt"
)

// InfraConfig is the infrastructure configuration section from vyx.yaml.
type InfraConfig struct {
	Backend     BackendConfig     `yaml:"backend"`
	Providers   []ProviderConfig  `yaml:"providers"`
	Stacks      []StackConfig     `yaml:"stacks"`
	DefaultTags map[string]string `yaml:"default_tags"`
}

// BackendConfig describes how and where state is stored.
type BackendConfig struct {
	Type   string         `yaml:"type"` // "local", "s3", "consul", "http"
	Config map[string]any `yaml:"config"`
}

// ProviderConfig describes a single provider to configure.
type ProviderConfig struct {
	Name    string         `yaml:"name"`
	Version string         `yaml:"version"`
	Config  map[string]any `yaml:"config"`
}

// StackConfig describes a named stack and its source directories.
type StackConfig struct {
	Name    string   `yaml:"name"`
	Sources []string `yaml:"sources"`
}

// DefaultInfraConfig returns an InfraConfig with sensible defaults.
func DefaultInfraConfig() InfraConfig {
	return InfraConfig{
		Backend: BackendConfig{
			Type: "local",
			Config: map[string]any{
				"path": ".vyx/infra.tfstate",
			},
		},
		Providers:   nil,
		Stacks:      nil,
		DefaultTags: map[string]string{},
	}
}

// Validate checks the infrastructure config for required fields.
func (c *InfraConfig) Validate() error {
	var errs []error

	if c.Backend.Type == "" {
		errs = append(errs, errors.New("infrastructure.backend.type is required"))
	} else {
		validTypes := map[string]bool{"local": true, "s3": true, "consul": true, "http": true}
		if !validTypes[c.Backend.Type] {
			errs = append(errs, fmt.Errorf("infrastructure.backend.type %q is invalid; use one of: local, s3, consul, http", c.Backend.Type))
		}
	}

	for i, p := range c.Providers {
		if p.Name == "" {
			errs = append(errs, fmt.Errorf("infrastructure.providers[%d].name is required", i))
		}
	}

	for i, s := range c.Stacks {
		if s.Name == "" {
			errs = append(errs, fmt.Errorf("infrastructure.stacks[%d].name is required", i))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
