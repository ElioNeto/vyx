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
		// S3 backend will be implemented in Phase 2.
		return nil, fmt.Errorf("s3 backend not yet implemented — use local backend")

	case "consul":
		// Consul backend will be implemented in Phase 3.
		return nil, fmt.Errorf("consul backend not yet implemented — use local backend")

	case "http":
		// HTTP backend will be implemented in Phase 4.
		return nil, fmt.Errorf("http backend not yet implemented — use local backend")

	default:
		return nil, fmt.Errorf("unknown backend type %q", cfg.Type)
	}
}
