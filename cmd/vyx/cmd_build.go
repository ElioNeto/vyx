package main

import (
	"flag"
	"fmt"
	"os"
)

func runBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	goDir := fs.String("go", "backend/go", "Directory containing Go source files")
	tsDir := fs.String("ts", "backend/node", "Directory containing TypeScript source files")
	frontendDir := fs.String("frontend", "frontend/src", "Directory containing React/TSX frontend files")
	output := fs.String("output", "route_map.json", "Output path for the generated route map")
	_ = fs.Parse(args)

	fmt.Println("\U0001f50d vyx build: scanning annotations...")

	if err := runBuildAnnotate(*goDir, *tsDir, *frontendDir, *output); err != nil {
		fmt.Fprintf(os.Stderr, "error: annotation scan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\u2705 route_map.json written to %s\n", *output)
	fmt.Println("\U0001f527 Building core binary...")

	if err := runCommand("go", "build", "-o", ".vyx/core", "./core/cmd/vyx"); err != nil {
		fmt.Fprintf(os.Stderr, "error: go build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\u2705 Build complete.")
}
