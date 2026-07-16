package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	infraapp "github.com/ElioNeto/vyx/core/application/infra"
	dinfra "github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/ElioNeto/vyx/core/infrastructure/infra/state"
	"github.com/ElioNeto/vyx/core/infrastructure/infra/templater"
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
	case "graph":
		runInfraGraph(ctx, subArgs)
	case "output":
		runInfraOutput(ctx, subArgs)
	case "import":
		runInfraImport(ctx, subArgs)
	case "state":
		runInfraState(ctx, subArgs)
	case "export":
		runInfraExport(ctx, subArgs)
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
  graph                  Generate dependency graph (mermaid or dot format)
  output                 Show output values from the state
  import                 Import existing cloud resource into state
  state                  Manage state (list, mv, rm, refresh)
  export                 Export to Terraform HCL or CloudFormation

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

// ─── vyx infra graph ────────────────────────────────────────────────────

func runInfraGraph(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("infra graph", flag.ExitOnError)
	format := fs.String("format", "mermaid", "Output format: mermaid or dot")
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

	gen := infraapp.NewGraphGenerator()
	result, err := gen.Generate(stack, infraapp.GraphFormat(*format))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: graph generation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}

// ─── vyx infra output ───────────────────────────────────────────────────

func runInfraOutput(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("infra output", flag.ExitOnError)
	formatJSON := fs.Bool("json", false, "Output in JSON format")
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

	collector := infraapp.NewOutputCollector(orch.Backend())

	// If a specific reference is given (e.g., "bucket.arn"), show just that.
	if len(args) > 0 && args[0][0] != '-' {
		ref := args[len(args)-1] // last arg is the reference
		val, err := collector.GetOutput(ctx, ref)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(val)
		return
	}

	// Otherwise show all outputs.
	entries, err := collector.AllOutputs(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *formatJSON {
		outputMap := make(map[string]map[string]string)
		for _, e := range entries {
			if outputMap[string(e.ResourceID)] == nil {
				outputMap[string(e.ResourceID)] = make(map[string]string)
			}
			outputMap[string(e.ResourceID)][e.Name] = e.Value
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(outputMap)
		return
	}

	if len(entries) == 0 {
		fmt.Println("No outputs found.")
		return
	}

	fmt.Println("Outputs:")
	for _, e := range entries {
		fmt.Printf("  %s.%s = %s\n", e.ResourceID, e.Name, e.Value)
	}
}

// ─── vyx infra import ───────────────────────────────────────────────────

func runInfraImport(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("infra import", flag.ExitOnError)
	cfg := parseInfraFlags(fs, args)

	if len(args) < 3 {
		fmt.Fprint(os.Stderr, "usage: vyx infra import <resource_type> <resource_id> <cloud_id>\n")
		fmt.Fprint(os.Stderr, "  resource_type  — e.g. aws_s3_bucket, aws_instance\n")
		fmt.Fprint(os.Stderr, "  resource_id    — vyx identifier for the resource\n")
		fmt.Fprint(os.Stderr, "  cloud_id       — provider-specific identifier (e.g. bucket name, instance ID)\n")
		os.Exit(1)
	}

	resourceType := dinfra.ResourceType(args[len(args)-3])
	resourceID := dinfra.ResourceID(args[len(args)-2])
	cloudID := args[len(args)-1]

	orch, err := setupInfraOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := orch.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: init failed: %v\n", err)
		os.Exit(1)
	}

	importer := infraapp.NewImporter(orch.Registry(), orch.Backend())
	result, err := importer.Import(ctx, resourceType, resourceID, cloudID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: import failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Imported %s %s (provider: %s, outputs: %d)\n",
		result.ResourceType, result.ResourceID, result.ProviderName, result.OutputCount)
}

// ─── vyx infra state ────────────────────────────────────────────────────

func runInfraState(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Fprint(os.Stderr, `vyx infra state — manage infrastructure state

Usage:
  vyx infra state list            List all resources in the state
  vyx infra state mv <from> <to>  Rename a resource in the state
  vyx infra state rm <id>         Remove a resource from the state
  vyx infra state refresh         Refresh outputs from the cloud
`)
		os.Exit(1)
	}

	subcommand := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("infra state "+subcommand, flag.ExitOnError)
	cfg := parseInfraFlags(fs, subArgs)

	orch, err := setupInfraOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := orch.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: init failed: %v\n", err)
		os.Exit(1)
	}

	sm := infraapp.NewStateManager(orch.Backend())

	switch subcommand {
	case "list":
		resources, err := sm.ListResources(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(resources) == 0 {
			fmt.Println("No resources in state.")
			return
		}
		fmt.Printf("Resources in state (%d):\n", len(resources))
		for _, r := range resources {
			fmt.Printf("  %s [%s] provider=%s state=%s\n", r.ID, r.Type, r.ProviderName, r.State)
		}

	case "mv":
		if len(subArgs) < 2 {
			fmt.Fprint(os.Stderr, "usage: vyx infra state mv <from> <to>\n")
			os.Exit(1)
		}
		from := dinfra.ResourceID(subArgs[len(subArgs)-2])
		to := dinfra.ResourceID(subArgs[len(subArgs)-1])
		if err := sm.MoveResource(ctx, from, to); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Moved %s → %s\n", from, to)

	case "rm":
		if len(subArgs) < 1 {
			fmt.Fprint(os.Stderr, "usage: vyx infra state rm <resource_id>\n")
			os.Exit(1)
		}
		id := dinfra.ResourceID(subArgs[len(subArgs)-1])
		if err := sm.RemoveResource(ctx, id); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Removed %s from state\n", id)

	case "refresh":
		if err := sm.Refresh(ctx, orch.Registry()); err != nil {
			fmt.Fprintf(os.Stderr, "error: refresh failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ State refreshed")

	default:
		fmt.Fprintf(os.Stderr, "error: unknown state subcommand %q\n", subcommand)
		os.Exit(1)
	}
}

// ─── vyx infra export ───────────────────────────────────────────────────

func runInfraExport(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("infra export", flag.ExitOnError)
	format := fs.String("format", "terraform", "Export format: terraform or cloudformation")
	output := fs.String("output", "", "Output file path (default: stdout)")
	region := fs.String("region", "us-east-1", "AWS region for Terraform provider")
	cfg := parseInfraFlags(fs, args)

	stack := createTestStack(cfg.stackName)

	switch *format {
	case "terraform", "tf":
		result, err := templater.ExportTerraform(stack, *region)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: export failed: %v\n", err)
			os.Exit(1)
		}
		if *output != "" {
			if err := os.WriteFile(*output, []byte(result), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error: write %s: %v\n", *output, err)
				os.Exit(1)
			}
			fmt.Printf("✓ Terraform HCL written to %s\n", *output)
		} else {
			fmt.Print(result)
		}

	case "cloudformation", "cfn":
		result, err := templater.ExportCloudFormation(stack)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: export failed: %v\n", err)
			os.Exit(1)
		}
		if *output != "" {
			if err := os.WriteFile(*output, []byte(result), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error: write %s: %v\n", *output, err)
				os.Exit(1)
			}
			fmt.Printf("✓ CloudFormation template written to %s\n", *output)
		} else {
			fmt.Print(result)
		}

	default:
		fmt.Fprintf(os.Stderr, "error: unsupported format %q (use 'terraform' or 'cloudformation')\n", *format)
		os.Exit(1)
	}
}
