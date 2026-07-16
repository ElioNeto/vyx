package main

import (
	"github.com/ElioNeto/vyx/core/domain/infra"
)

// registerProviders adds all available providers to the registry.
// Without any build tags, only the mock provider is registered.
func registerProviders(reg infra.ProviderRegistry, config map[string]any) error {
	mock := infra.NewMockProvider("mock")
	return reg.Register(mock)
}
