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
	if err := runAnnotate(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
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

// run contains the core annotation logic, extracted for testability
func run(goDir, tsDir, pyDir, frontendDir, output string) ([]error, error) {
	fmt.Println("vyx annotate: scanning for route annotations...")
	annotationErrs, err := scanner.Generate(goDir, tsDir, pyDir, frontendDir, output)

	// Convert []scanner.AnnotationError to []error
	errs := make([]error, len(annotationErrs))
	for i := range annotationErrs {
		errs[i] = &annotationErrs[i]
	}
	return errs, err
}
