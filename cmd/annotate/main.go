// Package main provides the `vyx annotate` CLI command.
// It scans backend (Go and TypeScript) and frontend directories for
// route annotations and generates a route_map.json file.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ElioNeto/vyx/scanner"
)

func main() {
	goDir := flag.String("go", "backend/go", "Directory containing Go source files")
	tsDir := flag.String("ts", "backend/node", "Directory containing TypeScript source files")
	pyDir := flag.String("py", "backend/python", "Directory containing Python source files")
	frontendDir := flag.String("frontend", "frontend/src", "Directory containing React/TSX frontend files")
	output := flag.String("output", "route_map.json", "Output path for the generated route map")
	flag.Parse()

	os.Exit(runMainWithExit(*goDir, *tsDir, *pyDir, *frontendDir, *output))
}

// runMainWithExit contains the main logic and returns an exit code for testability.
// Returns 0 on success, 1 on error.
func runMainWithExit(goDir, tsDir, pyDir, frontendDir, output string) int {
	if err := runMain(goDir, tsDir, pyDir, frontendDir, output); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		return 1
	}
	return 0
}

// runMain contains the main logic, extracted for testability.
func runMain(goDir, tsDir, pyDir, frontendDir, output string) error {
	return runAnnotateWithFlags(goDir, tsDir, pyDir, frontendDir, output)
}

// TestMain is exported for testing main() behavior.
// It simulates what main() does but returns error instead of calling os.Exit.
func TestMain() error {
	return runMain("backend/go", "backend/node", "backend/python", "frontend/src", "route_map.json")
}

// runAnnotate contains the CLI logic, extracted for testability
func runAnnotate() error {
	goDir := "backend/go"
	tsDir := "backend/node"
	pyDir := "backend/python"
	frontendDir := "frontend/src"
	output := "route_map.json"

	return runAnnotateWithFlags(goDir, tsDir, pyDir, frontendDir, output)
}

// runAnnotateWithFlags runs the annotate logic with the given parameters
func runAnnotateWithFlags(goDir, tsDir, pyDir, frontendDir, output string) error {
	annotationErrs, err := scanner.Generate(goDir, tsDir, pyDir, frontendDir, output)

	if err != nil {
		return err
	}

	if len(annotationErrs) > 0 {
		for _, e := range annotationErrs {
			os.Stderr.WriteString("annotation error: " + e.Error() + "\n")
		}
		return errors.New("annotation errors found")
	}

	fmt.Printf("route_map.json written to %s\n", output)
	return nil
}
