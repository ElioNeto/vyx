package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	infraapp "github.com/ElioNeto/vyx/core/application/infra"
	dinfra "github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/ElioNeto/vyx/core/infrastructure/infra/state"
)

func runInfra(args []string) {
	if len(args) < 1 {
		fmt.Fprint(os.Stderr, infraUsage)
		os.Exit(1)
	}

	subcommand := args[0]
	subArgs := args[1:]

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch subcommand {
	case "init":
		runInfraInit(ctx, subArgs)
	case "plan":
		runInfraPlan(ctx, subArgs)
	case "apply":
		runInfraApply(ctx, subArgs)
	case "destroy":
		runInfraDestroy(ctx, subArgs)
	case "-h", "--help", "help":
		fmt.Print(infraUsage)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown infra subcommand %q\n\n", subcommand)
		fmt.Fprint(os.Stderr, infraUsage)
		os.Exit(1)
	}
}

const infraUsage = `vyx infra — manage cloud infrastructure

Usage:
  vyx infra <command> [arguments]

Commands:
  init                   Initialize the state backend
  plan                   Show the plan (diff between desired and current state)
  apply                  Apply the planned changes
  destroy                Destroy all managed resources

Global flags:
  --state-path=<path>    Path to the state file (default: .vyx/infra.tfstate)
  --stack=<name>         Stack name (default: default)

Run 'vyx infra <command> -help' for command-specific flags.
`

// ─── helpers ─────────────────────────────────────────────────────────────

type infraConfig struct {
	statePath string
	stackName string
}

func parseInfraFlags(fs *flag.FlagSet, args []string) infraConfig {
	cfg := infraConfig{
		statePath: ".vyx/infra.tfstate",
		stackName: "default",
	}
	fs.StringVar(&cfg.statePath, "state-path", ".vyx/infra.tfstate", "Path to the state file")
	fs.StringVar(&cfg.stackName, "stack", "default", "Stack name")
	_ = fs.Parse(args)
	return cfg
}

func setupInfraOrchestrator(cfg infraConfig) (*infraapp.Orchestrator, error) {
	// Registry with mock provider for Phase 1.
	reg := dinfra.NewProviderRegistry()
	mock := dinfra.NewMockProvider("mock")
	if err := reg.Register(mock); err != nil {
		return nil, fmt.Errorf("register mock provider: %w", err)
	}

	// Local state backend.
	backend, err := state.NewLocalBackend(cfg.statePath)
	if err != nil {
		return nil, fmt.Errorf("create backend: %w", err)
	}

	return infraapp.NewOrchestrator(
		infraapp.NewPlanner(reg, backend),
		infraapp.NewApplier(reg, backend),
		infraapp.NewDestroyer(reg, backend),
		backend,
		reg,
	), nil
}

func createTestStack(name string) *dinfra.Stack {
	stack := dinfra.NewStack(name)
	r := dinfra.NewResource("bucket-1", "mock_resource", "mock")
	r.Properties = map[string]any{"name": "test-bucket", "size": 10}
	r.Tags = map[string]string{"env": "production"}
	stack.AddResource(r)

	r2 := dinfra.NewResource("queue-1", "mock_resource", "mock")
	r2.Properties = map[string]any{"name": "test-queue", "size": 5}
	r2.Tags = map[string]string{"env": "production"}
	r2.DependsOn = []dinfra.ResourceID{"bucket-1"}
	stack.AddResource(r2)

	return stack
}

// ─── vyx infra init ─────────────────────────────────────────────────────

func runInfraInit(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("infra init", flag.ExitOnError)
	cfg := parseInfraFlags(fs, args)

	orch, err := setupInfraOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := orch.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: init failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Infrastructure backend initialized (state: %s)\n", cfg.statePath)
}

// ─── vyx infra plan ─────────────────────────────────────────────────────

func runInfraPlan(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("infra plan", flag.ExitOnError)
	cfg := parseInfraFlags(fs, args)

	orch, err := setupInfraOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Ensure backend is initialized.
	if err := orch.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: init failed: %v\n", err)
		os.Exit(1)
	}

	stack := createTestStack(cfg.stackName)
	plan, err := orch.Plan(ctx, stack)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: plan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(plan.String())

	if plan.HasChanges() {
		for _, change := range plan.Changes {
			switch change.ChangeType {
			case dinfra.ChangeCreate:
				fmt.Printf("  + %s %s (create)\n", change.ResourceType, change.ResourceID)
			case dinfra.ChangeUpdate:
				fmt.Printf("  ~ %s %s (update)\n", change.ResourceType, change.ResourceID)
			case dinfra.ChangeDelete:
				fmt.Printf("  - %s %s (delete)\n", change.ResourceType, change.ResourceID)
			}
		}
		os.Exit(2) // exit code 2 means changes pending
	}
}

// ─── vyx infra apply ────────────────────────────────────────────────────

func runInfraApply(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("infra apply", flag.ExitOnError)
	autoApprove := fs.Bool("auto-approve", false, "Skip interactive approval")
	cfg := parseInfraFlags(fs, args)

	orch, err := setupInfraOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := orch.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: init failed: %v\n", err)
		os.Exit(1)
	}

	stack := createTestStack(cfg.stackName)

	// Acquire lock.
	if err := orch.AcquireLock(ctx, "apply"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = orch.ReleaseLock(ctx)
	}()

	// Plan first.
	plan, err := orch.Plan(ctx, stack)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: plan failed: %v\n", err)
		os.Exit(1)
	}

	if !plan.HasChanges() {
		fmt.Println("No changes to apply.")
		return
	}

	fmt.Println(plan.String())
	for _, change := range plan.Changes {
		switch change.ChangeType {
		case dinfra.ChangeCreate:
			fmt.Printf("  + %s %s\n", change.ResourceType, change.ResourceID)
		case dinfra.ChangeUpdate:
			fmt.Printf("  ~ %s %s\n", change.ResourceType, change.ResourceID)
		case dinfra.ChangeDelete:
			fmt.Printf("  - %s %s\n", change.ResourceType, change.ResourceID)
		}
	}

	// Interactive approval.
	if !*autoApprove {
		fmt.Print("\nDo you want to perform these actions? (yes/no): ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil || response != "yes" {
			fmt.Println("Apply cancelled.")
			return
		}
	}

	// Execute.
	newState, err := orch.Apply(ctx, plan, stack)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: apply failed: %v\n", err)
		os.Exit(1)
	}

	if newState != nil {
		fmt.Printf("✓ Applied %d changes (serial: %d)\n",
			plan.Summary.ToCreate+plan.Summary.ToUpdate+plan.Summary.ToDelete,
			newState.Serial)
	}
}

// ─── vyx infra destroy ──────────────────────────────────────────────────

func runInfraDestroy(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("infra destroy", flag.ExitOnError)
	autoApprove := fs.Bool("auto-approve", false, "Skip interactive approval")
	cfg := parseInfraFlags(fs, args)

	orch, err := setupInfraOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := orch.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: init failed: %v\n", err)
		os.Exit(1)
	}

	// Show what will be destroyed.
	currentState, err := orch.GetCurrentState(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load state: %v\n", err)
		os.Exit(1)
	}
	if currentState == nil || len(currentState.Resources) == 0 {
		fmt.Println("No resources to destroy.")
		return
	}

	fmt.Printf("Resources to destroy: %d\n", len(currentState.Resources))
	for _, r := range currentState.Resources {
		fmt.Printf("  - %s %s\n", r.Type, r.ID)
	}

	// Interactive approval.
	if !*autoApprove {
		fmt.Print("\nDo you really want to destroy all resources? (yes/no): ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil || response != "yes" {
			fmt.Println("Destroy cancelled.")
			return
		}
	}

	stack := createTestStack(cfg.stackName)
	if err := orch.Destroy(ctx, stack); err != nil {
		fmt.Fprintf(os.Stderr, "error: destroy failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ All resources destroyed.")
}
