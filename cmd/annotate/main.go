// Package main provides the `vyx annotate` CLI command.
// It scans backend (Go and TypeScript) and frontend directories for
// route annotations and generates a route_map.json file.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ElioNeto/vyx/scanner"
)

func main() {
	if err := runMain("backend/go", "backend/node", "backend/python", "frontend/src", "route_map.json"); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
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
