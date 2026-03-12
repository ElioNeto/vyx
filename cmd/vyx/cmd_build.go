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
	_ = fs.Parse(args)

	fmt.Println("🔍 vyx build: scanning annotations...")

	errs, err := scanner.Generate(*goDir, *tsDir, *frontendDir, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "annotation errors:")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  ❌ %s\n", e.Error())
		}
		os.Exit(1)
	}

	fmt.Printf("✅ route_map.json written to %s\n", *output)
	fmt.Println("🔧 Building core binary...")

	if err := runCommand("go", "build", "-o", ".vyx/core", "./core/cmd/vyx"); err != nil {
		fmt.Fprintf(os.Stderr, "error: go build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Build complete.")
}
