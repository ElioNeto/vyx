package config_test

import (
	"testing"
	"time"

	"github.com/ElioNeto/vyx/core/domain/config"
)

func TestConfig_Validate_EmptyProjectName(t *testing.T) {
	cfg := config.Config{
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty project name")
	}
}

func TestConfig_Validate_MissingWorkerID(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{Command: "echo test"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing worker ID")
	}
}

func TestConfig_Validate_MissingWorkerCommand(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing worker command")
	}
}

func TestConfig_Validate_NegativeReplicas(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Replicas: -1},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative replicas")
	}
}

func TestConfig_Validate_InvalidStrategy(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Strategy: "invalid"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid strategy")
	}
}

func TestConfig_Validate_NegativeShutdownTimeout(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", ShutdownTimeout: -1 * time.Second},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative shutdown timeout")
	}
}

func TestConfig_Validate_NegativeCircuitBreakerFailures(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Security: config.SecurityConfig{
			CircuitBreaker: config.CircuitBreakerConfig{
				Failures: -1,
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative circuit breaker failures")
	}
}

func TestConfig_Validate_NegativeCircuitBreakerCooldown(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Security: config.SecurityConfig{
			CircuitBreaker: config.CircuitBreakerConfig{
				Cooldown: -1 * time.Second,
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative circuit breaker cooldown")
	}
}

func TestConfig_Validate_NegativeCircuitBreakerHalfOpenMax(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Security: config.SecurityConfig{
			CircuitBreaker: config.CircuitBreakerConfig{
				HalfOpenMax: -1,
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative circuit breaker half_open_max")
	}
}

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Replicas: 1},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := config.Defaults()

	if cfg.Project.Version == "" {
		t.Error("expected version in defaults")
	}
	if cfg.Security.JWTSecretEnv != "JWT_SECRET" {
		t.Errorf("expected JWT_SECRET, got %s", cfg.Security.JWTSecretEnv)
	}
	if cfg.Security.CircuitBreaker.Failures != 5 {
		t.Errorf("expected 5 failures, got %d", cfg.Security.CircuitBreaker.Failures)
	}
	if cfg.Security.CircuitBreaker.Cooldown != 30*time.Second {
		t.Errorf("expected 30s cooldown, got %v", cfg.Security.CircuitBreaker.Cooldown)
	}
	if cfg.Security.CircuitBreaker.HalfOpenMax != 3 {
		t.Errorf("expected 3 half_open_max, got %d", cfg.Security.CircuitBreaker.HalfOpenMax)
	}
}

func TestConfig_SecurityDefaults(t *testing.T) {
	cfg := config.Defaults()

	if cfg.Security.RateLimit.PerIP != 100 {
		t.Errorf("expected 100 per_ip, got %d", cfg.Security.RateLimit.PerIP)
	}
	if cfg.Security.RateLimit.PerToken != 500 {
		t.Errorf("expected 500 per_token, got %d", cfg.Security.RateLimit.PerToken)
	}
	if cfg.Security.PayloadMaxSize != "1mb" {
		t.Errorf("expected 1mb, got %s", cfg.Security.PayloadMaxSize)
	}
	if cfg.Security.GlobalTimeout != 30*time.Second {
		t.Errorf("expected 30s, got %v", cfg.Security.GlobalTimeout)
	}
}

func TestConfig_IPCDefaults(t *testing.T) {
	cfg := config.Defaults()

	if cfg.IPC.SocketDir != "/tmp/vyx" {
		t.Errorf("expected /tmp/vyx, got %s", cfg.IPC.SocketDir)
	}
	if cfg.IPC.ArrowThreshold != "512kb" {
		t.Errorf("expected 512kb, got %s", cfg.IPC.ArrowThreshold)
	}
}

func TestConfig_BuildDefaults(t *testing.T) {
	cfg := config.Defaults()

	if cfg.Build.SchemasDir != "./schemas" {
		t.Errorf("expected ./schemas, got %s", cfg.Build.SchemasDir)
	}
	if cfg.Build.RouteMapOutput != "./route_map.json" {
		t.Errorf("expected ./route_map.json, got %s", cfg.Build.RouteMapOutput)
	}
}

func TestConfig_Validate_ValidReplicas(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Replicas: 3},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_ZeroReplicas(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Replicas: 0},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_ValidStrategy(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Strategy: "round-robin"},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_LeastLoadedStrategy(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Strategy: "least-loaded"},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_EmptyStrategy(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Strategy: ""},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_ValidPoolSize(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", PoolSize: 10},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_ZeroPoolSize(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", PoolSize: 0},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_NegativePoolSize(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", PoolSize: -1},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative pool_size")
	}
}

func TestConfig_Validate_MultipleWorkers(t *testing.T) {
	cfg := config.Config{
		Project: config.ProjectConfig{Name: "test"},
		Workers: []config.WorkerConfig{
			{ID: "worker1", Command: "echo", Replicas: 2, Strategy: "round-robin", PoolSize: 5},
			{ID: "worker2", Command: "node", Replicas: 3, Strategy: "least-loaded", PoolSize: 10},
		},
		Security: config.SecurityConfig{
			JWTSecretEnv: "JWT_SECRET",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
