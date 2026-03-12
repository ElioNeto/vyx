package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/ElioNeto/vyx/scanner"
)

func runAnnotate(args []string) {
	fs := flag.NewFlagSet("annotate", flag.ExitOnError)
	goDir := fs.String("go", "backend/go", "Directory containing Go source files")
	tsDir := fs.String("ts", "backend/node", "Directory containing TypeScript source files")
	frontendDir := fs.String("frontend", "frontend/src", "Directory containing React/TSX frontend files")
	jsonOut := fs.Bool("json", false, "Output discovered routes as JSON instead of table")
	_ = fs.Parse(args)

	routes, errs := collectRoutes(*goDir, *tsDir, *frontendDir)

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "annotation errors:")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  ❌ %s\n", e.Error())
		}
		os.Exit(1)
	}

	if len(routes) == 0 {
		fmt.Println("No routes found.")
		return
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(map[string]any{"routes": routes}, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Human-readable table.
	fmt.Printf("%-8s %-35s %-20s %s\n", "METHOD", "PATH", "WORKER", "AUTH")
	fmt.Println("------------------------------------------------------------------------")
	for _, r := range routes {
		auth := "-"
		if len(r.AuthRoles) > 0 {
			for i, role := range r.AuthRoles {
				if i > 0 {
					auth += ","
				}
				auth = role
			}
		}
		fmt.Printf("%-8s %-35s %-20s %s\n", r.Method, r.Path, r.WorkerID, auth)
	}
	fmt.Printf("\n%d route(s) discovered.\n", len(routes))
}

// collectRoutes runs the scanner across all three directories and returns
// routes + any annotation errors.
func collectRoutes(goDir, tsDir, frontendDir string) ([]scanner.Route, []scanner.AnnotationError) {
	var routes []scanner.Route
	var errs []scanner.AnnotationError

	if goDir != "" {
		r, e := scanner.ParseGoFiles(goDir, "go:"+goDir)
		routes = append(routes, r...)
		errs = append(errs, e...)
	}
	if tsDir != "" {
		r, e := scanner.ParseTSFiles(tsDir, "node:"+tsDir)
		routes = append(routes, r...)
		errs = append(errs, e...)
	}
	if frontendDir != "" {
		r, e := scanner.ParseTSFiles(frontendDir, "node:frontend")
		routes = append(routes, r...)
		errs = append(errs, e...)
	}

	validErrs := scanner.Validate(routes)
	errs = append(errs, validErrs...)

	return routes, errs
}

// runCommand is a shared helper to exec a process and stream output.
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
