package state

import (
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// NewBackend creates a Backend based on the given configuration.
func NewBackend(cfg infra.BackendConfig) (infra.Backend, error) {
	switch cfg.Type {
	case "local":
		path := ".vyx/infra.tfstate"
		if p, ok := cfg.Config["path"].(string); ok && p != "" {
			path = p
		}
		return NewLocalBackend(path)

	case "s3":
		return newS3Backend(cfg)

	case "consul":
		addr := "http://localhost:8500"
		if v, ok := cfg.Config["address"].(string); ok && v != "" {
			addr = v
		}
		path := "vyx/infra/tfstate"
		if v, ok := cfg.Config["path"].(string); ok && v != "" {
			path = v
		}
		return NewConsulBackend(ConsulBackendConfig{
			Addr:       addr,
			Path:       path,
			Datacenter: getString(cfg.Config, "datacenter"),
		})

	case "http":
		addr := ""
		if v, ok := cfg.Config["address"].(string); ok && v != "" {
			addr = v
		}
		if addr == "" {
			return nil, fmt.Errorf("http backend: address is required")
		}
		return NewHTTPBackend(HTTPBackendConfig{Addr: addr}), nil

	default:
		return nil, fmt.Errorf("unknown backend type %q", cfg.Type)
	}
}

// getString retrieves a string value from a map with a type assertion.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}


