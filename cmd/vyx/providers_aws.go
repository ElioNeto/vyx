//go:build with_aws
// +build with_aws

package main

import (
	"github.com/ElioNeto/vyx/core/domain/infra"
	awsprovider "github.com/ElioNeto/vyx/core/infrastructure/infra/providers/aws"
)

// registerProviders adds all available providers to the registry.
// When built with -tags with_aws, the real AWS provider is registered.
func registerProviders(reg infra.ProviderRegistry, config map[string]any) error {
	// Always register mock for testing
	mock := infra.NewMockProvider("mock")
	if err := reg.Register(mock); err != nil {
		return err
	}

	// Register real AWS provider
	awsConfig, _ := config["aws"].(map[string]any)
	if awsConfig == nil {
		awsConfig = make(map[string]any)
	}
	awsProvider, err := awsprovider.New(awsConfig)
	if err != nil {
		return err
	}
	return reg.Register(awsProvider)
}
