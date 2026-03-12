// Command vyx is the unified CLI for the vyx framework.
//
// Subcommands:
//
//	vyx new project <name>  — scaffold a new vyx project
//	vyx dev                 — start core in development mode
//	vyx build               — run scanner and build all artefacts
//	vyx annotate            — validate annotations and print route map
package main

import (
	"fmt"
	"os"
)

const usage = `vyx — the polyglot microframework CLI

Usage:
  vyx <command> [arguments]

Commands:
  new project <name>   Scaffold a new vyx project in ./<name>/
  dev                  Start the core in development mode (hot reload)
  build                Run the annotation scanner and build all artefacts
  annotate             Validate annotations and print the route map to stdout

Flags:
  -h, --help           Show this help message

Run 'vyx <command> -help' for command-specific flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "new":
		runNew(os.Args[2:])
	case "dev":
		runDev(os.Args[2:])
	case "build":
		runBuild(os.Args[2:])
	case "annotate":
		runAnnotate(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", os.Args[1])
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}
