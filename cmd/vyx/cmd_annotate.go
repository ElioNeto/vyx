package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
func runAnnotateCmd(goDir, tsDir, frontendDir, output string) error {
	// Make all paths absolute so they resolve correctly when the annotate
	// binary runs from the vyx source directory.
	absGo, _ := filepath.Abs(goDir)
	absTs, _ := filepath.Abs(tsDir)
	absFrontend, _ := filepath.Abs(frontendDir)
	absOutput, _ := filepath.Abs(output)

	// Try installed binary first.
	path, err := exec.LookPath("vyx-annotate")
	if err != nil {
		path = ""
	}

	var cmd *exec.Cmd
	if path != "" {
		cmd = exec.Command(path,
			"-go", absGo,
			"-ts", absTs,
			"-frontend", absFrontend,
			"-output", absOutput,
		)
	} else {
		// Locate cmd/annotate using the vyx source directory.
		annotatePkg, vyxSrc := findAnnotatePkg()
		if vyxSrc != "" {
			cmd = exec.Command("go", "run", annotatePkg,
				"-go", absGo,
				"-ts", absTs,
				"-frontend", absFrontend,
				"-output", absOutput,
			)
			cmd.Dir = vyxSrc
		} else {
			cmd = exec.Command("go", "run", annotatePkg,
				"-go", absGo,
				"-ts", absTs,
				"-frontend", absFrontend,
				"-output", absOutput,
			)
		}
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// findAnnotatePkg returns the Go package path for cmd/annotate
// and the vyx source directory to run it from.
func findAnnotatePkg() (pkg, srcDir string) {
	// Check if the annotate package exists locally (monorepo / dev setup).
	if _, err := os.Stat("cmd/annotate/main.go"); err == nil {
		return "./cmd/annotate", ""
	}
	// Try to find vyx source directory.
	srcDir = findVyxSource()
	if srcDir != "" {
		if _, err := os.Stat(filepath.Join(srcDir, "cmd", "annotate", "main.go")); err == nil {
			return "./cmd/annotate", srcDir
		}
	}
	return "github.com/ElioNeto/vyx/cmd/annotate", srcDir
}

// runCommand is a shared helper to exec a process and stream its output.
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
