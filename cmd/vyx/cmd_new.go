package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const newUsage = `Usage: vyx new project <name>

Scaffolds a new vyx project directory at ./<name>/ with the following structure:

  <name>/
  ├── core/
  ├── backend/
  │   ├── go/
  │   ├── node/
  │   └── python/
  ├── frontend/
  │   └── src/
  ├── schemas/
  └── vyx.yaml

Flags:
  -h, --help   Show this help message
`

// defaultVyxYAML returns the content of a sensible default vyx.yaml for projectName.
// Values are sourced from the canonical defaults in core/domain/config.
func defaultVyxYAML(projectName string) string {
	return fmt.Sprintf(`project:
  name: %s
  version: 0.1.0

workers:
  - id: node:api
    command: node backend/node/index.js
    replicas: 1
    strategy: round-robin
    startup_timeout: 10s
    shutdown_timeout: 5s

security:
  jwt_secret_env: JWT_SECRET
  rate_limit:
    per_ip: 100
    per_token: 500
  payload_max_size: 1mb
  global_timeout: 30s

ipc:
  socket_dir: /tmp/vyx
  arrow_threshold: 512kb

build:
  schemas_dir: ./schemas
  route_map_output: ./route_map.json
`, projectName)
}

func runNew(args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(newUsage)
		if len(args) == 0 {
			os.Exit(1)
		}
		return
	}

	if args[0] != "project" {
		fmt.Fprintf(os.Stderr, "error: unknown subcommand %q for 'vyx new'\n", args[0])
		fmt.Fprintf(os.Stderr, "Did you mean: vyx new project <name>\n")
		os.Exit(1)
	}

	if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
		fmt.Fprintln(os.Stderr, "error: project name is required")
		fmt.Fprintln(os.Stderr, "Usage: vyx new project <name>")
		os.Exit(1)
	}

	name := strings.TrimSpace(args[1])
	if err := scaffoldProject(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Project %q created at ./%s/\n", name, name)
	fmt.Printf("   Next steps:\n")
	fmt.Printf("     cd %s\n", name)
	fmt.Printf("     vyx dev\n")
}

// scaffoldProject creates the project directory tree and vyx.yaml.
func scaffoldProject(name string) error {
	root := filepath.Join(".", name)

	if _, err := os.Stat(root); err == nil {
		return fmt.Errorf("directory %q already exists", root)
	}

	dirs := []string{
		"core",
		"backend/go",
		"backend/node",
		"backend/python",
		"frontend/src",
		"schemas",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	// Write vyx.yaml seeded from defaults.
	vyxYAML := defaultVyxYAML(name)

	// Validate the generated YAML parses correctly before writing.
	var check any
	if err := yaml.Unmarshal([]byte(vyxYAML), &check); err != nil {
		return fmt.Errorf("generated vyx.yaml is invalid: %w", err)
	}

	yamlPath := filepath.Join(root, "vyx.yaml")
	if err := os.WriteFile(yamlPath, []byte(vyxYAML), 0644); err != nil {
		return fmt.Errorf("write vyx.yaml: %w", err)
	}

	// Write .gitkeep files so empty directories are tracked by git.
	for _, d := range []string{"backend/go", "backend/node", "backend/python", "frontend/src", "schemas"} {
		gitkeep := filepath.Join(root, d, ".gitkeep")
		_ = os.WriteFile(gitkeep, []byte{}, 0644)
	}

	return nil
}
