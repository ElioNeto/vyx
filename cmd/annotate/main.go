// Package main provides the `vyx annotate` CLI command.
// It scans backend (Go and TypeScript) and frontend directories for
// route annotations and generates a route_map.json file.
package main

import (
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

	errs, err := run(*goDir, *tsDir, *pyDir, *frontendDir, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "annotation errors found:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  %s\n", e.Error())
		}
		os.Exit(1)
	}

	fmt.Printf("route_map.json written to %s\n", *output)
}

// run contains the core annotation logic, extracted for testability
func run(goDir, tsDir, pyDir, frontendDir, output string) ([]error, error) {
	fmt.Println("vyx annotate: scanning for route annotations...")
	annotationErrs, err := scanner.Generate(goDir, tsDir, pyDir, frontendDir, output)
	
	// Convert []scanner.AnnotationError to []error (AnnotationError implements error via pointer receiver)
	errs := make([]error, len(annotationErrs))
	for i := range annotationErrs {
		errs[i] = &annotationErrs[i] // Use pointer to get *AnnotationError which has Error() method
	}
	return errs, err
}
