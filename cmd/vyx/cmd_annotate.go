package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func runAnnotate(args []string) {
	fs := flag.NewFlagSet("annotate", flag.ExitOnError)
	goDir := fs.String("go", "backend/go", "Directory containing Go source files")
	tsDir := fs.String("ts", "backend/node", "Directory containing TypeScript source files")
	frontendDir := fs.String("frontend", "frontend/src", "Directory containing React/TSX frontend files")
	output := fs.String("output", "route_map.json", "Output path")
	_ = fs.Parse(args)

	fmt.Println("\U0001f50d vyx annotate: scanning for route annotations...")

	if err := runAnnotateCmd(*goDir, *tsDir, *frontendDir, *output); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runBuildAnnotate(goDir, tsDir, frontendDir, output string) error {
	return runAnnotateCmd(goDir, tsDir, frontendDir, output)
}

// runAnnotateCmd delegates annotation scanning to the cmd/annotate binary.
// It first looks for a pre-installed `vyx-annotate` on PATH, then falls back
// to `go run` so no separate install is required during development.
func runAnnotateCmd(goDir, tsDir, frontendDir, output string) error {
	// Try installed binary first.
	path, err := exec.LookPath("vyx-annotate")
	if err != nil {
		// Fall back to go run relative to the project root.
		// vyx is always run from the project root, so ../../ is not correct here;
		// the annotate cmd lives at <repo>/cmd/annotate relative to the repo root.
		// We resolve it relative to the vyx binary location.
		path = ""
	}

	var cmd *exec.Cmd
	if path != "" {
		cmd = exec.Command(path,
			"-go", goDir,
			"-ts", tsDir,
			"-frontend", frontendDir,
			"-output", output,
		)
	} else {
		// Locate cmd/annotate relative to the working directory (project root).
		annotatePkg := findAnnotatePkg()
		cmd = exec.Command("go", "run", annotatePkg,
			"-go", goDir,
			"-ts", tsDir,
			"-frontend", frontendDir,
			"-output", output,
		)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// findAnnotatePkg returns the Go package path for cmd/annotate.
// When running from a vyx project root (scaffolded by vyx new), the
// repo is not present locally — so we try the installed module path.
func findAnnotatePkg() string {
	// Check if the annotate package exists locally (monorepo / dev setup).
	if _, err := os.Stat("cmd/annotate/main.go"); err == nil {
		return "./cmd/annotate"
	}
	return "github.com/ElioNeto/vyx/cmd/annotate"
}

// runCommand is a shared helper to exec a process and stream its output.
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
