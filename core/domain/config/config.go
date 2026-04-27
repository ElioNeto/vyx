// Package config defines the domain schema for the vyx.yaml project manifest.
// This is a pure value-object layer — no I/O, no external dependencies beyond stdlib.
package config

import (
	"errors"
	"fmt"
	"time"
)

// Config is the top-level representation of vyx.yaml.
type Config struct {
	Project  ProjectConfig  `yaml:"project"`
	Workers  []WorkerConfig `yaml:"workers"`
	Security SecurityConfig `yaml:"security"`
	IPC      IPCConfig      `yaml:"ipc"`
	Build    BuildConfig    `yaml:"build"`
}

// ProjectConfig holds project metadata.
type ProjectConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// WorkerConfig describes a single managed worker process.
type WorkerConfig struct {
	ID              string        `yaml:"id"`
	Command         string        `yaml:"command"`
	WorkingDir      string        `yaml:"working_dir"`      // optional: directory to run the command in
	RuntimeVersion  string        `yaml:"runtime_version"` // optional: e.g. "20", "3.12"
	Replicas        int           `yaml:"replicas"`
	Strategy        string        `yaml:"strategy"`         // "round-robin" | "least-loaded"
	StartupTimeout  time.Duration `yaml:"startup_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"` // max time to wait for in-flight requests
}

// SecurityConfig holds JWT, rate-limiting and payload settings.
type SecurityConfig struct {
	JWTSecretEnv    string              `yaml:"jwt_secret_env"`
	RateLimit       RateLimitConfig    `yaml:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	PayloadMaxSize string              `yaml:"payload_max_size"` // e.g. "1mb"
	GlobalTimeout  time.Duration      `yaml:"global_timeout"`
}

// RateLimitConfig defines per-IP and per-token request caps.
type RateLimitConfig struct {
	PerIP    int `yaml:"per_ip"`
	PerToken int `yaml:"per_token"`
}

// CircuitBreakerConfig defines failure thresholds and cooldown.
type CircuitBreakerConfig struct {
	Failures   int           `yaml:"failures"`    // consecutive failures before opening
	Cooldown  time.Duration `yaml:"cooldown"`   // time in open state before half-open
	HalfOpenMax int         `yaml:"half_open_max"` // max probes in half-open state
}

// IPCConfig holds Unix Domain Socket and Arrow settings.
type IPCConfig struct {
	SocketDir      string `yaml:"socket_dir"`
	ArrowThreshold string `yaml:"arrow_threshold"` // e.g. "512kb"
}

// BuildConfig holds paths used at build time by the annotation scanner.
type BuildConfig struct {
	SchemasDir     string `yaml:"schemas_dir"`
	RouteMapOutput string `yaml:"route_map_output"`
}

// Defaults returns a Config pre-filled with sensible production defaults.
func Defaults() Config {
	return Config{
		Project: ProjectConfig{
			Version: "0.2.0",
		},
		Security: SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
			RateLimit: RateLimitConfig{
				PerIP:    100,
				PerToken: 500,
			},
			CircuitBreaker: CircuitBreakerConfig{
				Failures:    5,
				Cooldown:   30 * time.Second,
				HalfOpenMax: 3,
			},
			PayloadMaxSize: "1mb",
			GlobalTimeout:  30 * time.Second,
		},
		IPC: IPCConfig{
			SocketDir:      "/tmp/vyx",
			ArrowThreshold: "512kb",
		},
		Build: BuildConfig{
			SchemasDir:     "./schemas",
			RouteMapOutput: "./route_map.json",
		},
	}
}

// Validate checks the config for required fields and invalid combinations.
// Returns a joined error listing every problem found.
func (c *Config) Validate() error {
	var errs []error

	if c.Project.Name == "" {
		errs = append(errs, errors.New("project.name is required"))
	}

	for i, w := range c.Workers {
		if w.ID == "" {
			errs = append(errs, fmt.Errorf("workers[%d].id is required", i))
		}
		if w.Command == "" {
			errs = append(errs, fmt.Errorf("workers[%d].command is required (id: %q)", i, w.ID))
		}
		if w.Replicas < 0 {
			errs = append(errs, fmt.Errorf("workers[%d].replicas must be >= 0 (id: %q)", i, w.ID))
		}
		validStrategies := map[string]bool{"round-robin": true, "least-loaded": true, "": true}
		if !validStrategies[w.Strategy] {
			errs = append(errs, fmt.Errorf("workers[%d].strategy %q is invalid; use \"round-robin\" or \"least-loaded\" (id: %q)", i, w.Strategy, w.ID))
		}
		if w.ShutdownTimeout < 0 {
			errs = append(errs, fmt.Errorf("workers[%d].shutdown_timeout must be >= 0 (id: %q)", i, w.ID))
		}
	}

	if c.Security.CircuitBreaker.Failures < 0 {
		errs = append(errs, errors.New("security.circuit_breaker.failures must be >= 0"))
	}
	if c.Security.CircuitBreaker.Cooldown < 0 {
		errs = append(errs, errors.New("security.circuit_breaker.cooldown must be >= 0"))
	}
	if c.Security.CircuitBreaker.HalfOpenMax < 0 {
		errs = append(errs, errors.New("security.circuit_breaker.half_open_max must be >= 0"))
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
