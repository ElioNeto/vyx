package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ElioNeto/vyx/scanner"
)

func runBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	goDir := fs.String("go", "backend/go", "Directory containing Go source files")
	tsDir := fs.String("ts", "backend/node", "Directory containing TypeScript source files")
	frontendDir := fs.String("frontend", "frontend/src", "Directory containing React/TSX frontend files")
	output := fs.String("output", "route_map.json", "Output path for the generated route map")
	infraDir := fs.String("infra", "infra", "Directory containing infrastructure resource definitions")
	infraOutput := fs.String("infra-output", "infra_map.json", "Output path for the generated infra map")
	_ = fs.Parse(args)

	fmt.Println("\U0001f50d vyx build: scanning annotations...")

	if err := runBuildAnnotate(*goDir, *tsDir, *frontendDir, *output); err != nil {
		fmt.Fprintf(os.Stderr, "error: annotation scan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\u2705 route_map.json written to %s\n", *output)

	// Scan infrastructure annotations if infra directory exists
	if *infraDir != "" {
		if _, err := os.Stat(*infraDir); err == nil {
			fmt.Println("\U0001f50d vyx build: scanning infrastructure annotations...")
			infraErrs, err := scanner.GenerateInfra(*infraDir, *infraOutput)
			if len(infraErrs) > 0 {
			for _, e := range infraErrs {
				fmt.Fprintf(os.Stderr, "  warning: %v\n", e)
			}
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: infra scan failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("\u2705 infra_map.json written to %s\n", *infraOutput)
		}
	}

	fmt.Println("\U0001f527 Building core binary...")

	if err := runCommand("go", "build", "-buildvcs=false", "-o", ".vyx/core", "github.com/ElioNeto/vyx/core/cmd/vyx"); err != nil {
		fmt.Fprintf(os.Stderr, "error: go build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\u2705 Build complete.")
}
