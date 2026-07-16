// Package main — vyx install command
// Installs all worker dependencies into .vyx/deps/<runtime>/
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// depsDir is the root for all installed dependencies.
const depsDir = ".vyx/deps"

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	configPath := fs.String("config", "vyx.yaml", "Path to vyx.yaml")
	workerID := fs.String("worker", "", "Install deps for a specific worker only")
	frontendDir := fs.String("frontend", "frontend", "Frontend directory (Vite)")
	_ = fs.Parse(args)

	if _, err := os.Stat(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: config file not found: %s\n", *configPath)
		os.Exit(1)
	}

	cfg, err := loadFullConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		os.Exit(1)
	}

	// Install worker deps
	for _, w := range cfg.Workers {
		if *workerID != "" && w.ID != *workerID {
			continue
		}
		fmt.Printf("📦 Installing deps for %s (%s)...\n", w.ID, w.Command)
		if err := installWorkerDeps(w); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  %v\n", err)
		} else {
			fmt.Printf("  ✅ %s ready\n", w.ID)
		}
	}

	// Install frontend deps centrally
	if *workerID == "" {
		if err := installFrontendDeps(*frontendDir); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  frontend: %v\n", err)
		}
	}

	fmt.Println("\n✅ All dependencies installed.")
}

// installFrontendDeps installs frontend deps into .vyx/deps/ts-js/node_modules
func installFrontendDeps(frontendDir string) error {
	pkgPath := filepath.Join(frontendDir, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return fmt.Errorf("no package.json found in %s", frontendDir)
	}

	targetDir := filepath.Join(depsDir, "ts-js")
	os.MkdirAll(targetDir, 0755)
	depsNM := filepath.Join(targetDir, "node_modules")
	frontendNM := filepath.Join(frontendDir, "node_modules")

	// Check if already installed centrally by looking for a frontend-specific package
	if _, err := os.Stat(filepath.Join(depsNM, "react")); err == nil {
		ensureSymlink(frontendNM, depsNM)
		return nil
	}

	// Temporarily remove symlink so npm install creates a real dir
	os.Remove(frontendNM)

	// Install in frontend dir
	fmt.Println("   Installing frontend deps...")
	cmd := exec.Command("npm", "install", "--no-package-lock", "--no-audit", "--no-fund")
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}

	// If deps node_modules already exists (from backend), merge by copying
	if _, err := os.Stat(depsNM); err == nil {
		copyMerged(frontendNM, depsNM)
		os.RemoveAll(frontendNM)
	} else {
		if err := os.Rename(frontendNM, depsNM); err != nil {
			return fmt.Errorf("move node_modules: %w", err)
		}
	}

	// Symlink back
	ensureSymlink(frontendNM, depsNM)
	return nil
}

func ensureSymlink(linkPath, targetPath string) {
	os.Remove(linkPath)
	rel, _ := filepath.Rel(filepath.Dir(linkPath), targetPath)
	os.Symlink(rel, linkPath)
}

// copyMerged copies packages from src to dst, merging node_modules dirs.
// Also merges .bin/ directory for CLI tools (vite, tsc, etc).
func copyMerged(src, dst string) {
	entries, _ := os.ReadDir(src)
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		// Always merge .bin directory
		if e.Name() == ".bin" && e.IsDir() {
			mergeBinDir(srcPath, dstPath)
			continue
		}
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			os.Rename(srcPath, dstPath)
		}
	}
}

// mergeBinDir merges CLI binaries from srcBin to dstBin
func mergeBinDir(srcBin, dstBin string) {
	os.MkdirAll(dstBin, 0755)
	entries, _ := os.ReadDir(srcBin)
	for _, e := range entries {
		srcPath := filepath.Join(srcBin, e.Name())
		dstPath := filepath.Join(dstBin, e.Name())
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			os.Rename(srcPath, dstPath)
		}
	}
}

// workerConfigFull matches the worker section in vyx.yaml.
type workerConfigFull struct {
	ID              string `yaml:"id"`
	Command         string `yaml:"command"`
	WorkingDir      string `yaml:"working_dir"`
	RuntimeVersion string `yaml:"runtime_version"`
}

// fullConfig matches the full vyx.yaml structure.
type fullConfig struct {
	Workers []workerConfigFull `yaml:"workers"`
}

func loadFullConfig(path string) (*fullConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg fullConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func installWorkerDeps(w workerConfigFull) error {
	workDir := w.WorkingDir
	if workDir == "" {
		workDir = "."
	}

	// Detect runtime from command
	rt := detectRuntime(w.Command)

	switch rt {
	case "node":
		return installNodeDeps(workDir)
	case "go":
		return installGoDeps(workDir)
	case "python":
		return installPythonDeps(workDir)
	default:
		fmt.Printf("  ℹ️  Unknown runtime for %s — skipping\n", w.ID)
		return nil
	}
}

func detectRuntime(command string) string {
	parts := splitCommand(command)
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "node", "npm", "npx":
		return "node"
	case "go":
		return "go"
	case "python", "python3":
		return "python"
	default:
		return ""
	}
}

func splitCommand(cmd string) []string {
	var result []string
	current := ""
	inQuote := false
	for _, ch := range cmd {
		switch {
		case ch == '"' || ch == '\'':
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current != "" {
				result = append(result, current)
				current = ""
			}
		default:
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// ─── Node.js deps ──────────────────────────────────────────────────────

func installNodeDeps(workDir string) error {
	pkgPath := filepath.Join(workDir, "package.json")
	if _, err := os.Stat(pkgPath); err != nil {
		return fmt.Errorf("no package.json found in %s", workDir)
	}

	targetDir := filepath.Join(depsDir, "ts-js")
	os.MkdirAll(targetDir, 0755)
	depsNM := filepath.Join(targetDir, "node_modules")
	workNM := filepath.Join(workDir, "node_modules")

	// Remove symlink so npm creates real dir
	os.Remove(workNM)

	// Run npm install in the worker's working directory
	fmt.Println("   Running npm install in", workDir)
	cmd := exec.Command("npm", "install", "--no-package-lock", "--no-audit", "--no-fund")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}

	// Before moving, rebase symlinks to resolve from new location
	rebaseSymlinks(workNM, depsNM)

	// If deps node_modules exists, merge
	if _, err := os.Stat(depsNM); err == nil {
		copyMerged(workNM, depsNM)
		os.RemoveAll(workNM)
	} else {
		if err := os.Rename(workNM, depsNM); err != nil {
			return nil // leave in place if cross-device
		}
	}

	// Symlink back
	ensureSymlink(workNM, depsNM)
	return nil
}

// rebaseSymlinks rewrites symlink targets in src to work from dst.
// npm creates symlinks with relative paths in node_modules (e.g. file: deps).
// When node_modules is moved, those relative paths break. This function
// rebases them by computing new relative paths from the new location.
func rebaseSymlinks(srcDir, dstDir string) {
	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		target, err := os.Readlink(path)
		if err != nil || filepath.IsAbs(target) {
			return nil
		}
		// Only rebase relative symlinks (file: deps use relative paths)
		// Old: from srcDir/sub/node_modules/pkg -> ../../target
		// New: from dstDir/sub/node_modules/pkg -> (recalculated)
		srcDir2 := filepath.Dir(path)
		dstPath := filepath.Join(dstDir, path[len(srcDir)+1:])
		dstDir2 := filepath.Dir(dstPath)

		// Resolve the old target to an absolute path
		absTarget := filepath.Join(srcDir2, target)
		absTarget, err = filepath.EvalSymlinks(absTarget)
		if err != nil {
			return nil
		}

		// Compute new relative path from the new location
		newTarget, err := filepath.Rel(dstDir2, absTarget)
		if err != nil {
			return nil
		}

		// Update the symlink
		os.Remove(path)
		os.Symlink(newTarget, path)
		return nil
	})
}

// ─── Go deps ──────────────────────────────────────────────────────────

func installGoDeps(workDir string) error {
	goModPath := filepath.Join(workDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("no go.mod found in %s", workDir)
	}

	targetDir := filepath.Join(depsDir, "go")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create deps dir: %w", err)
	}

	// Run go mod download
	cmd := exec.Command("go", "mod", "download")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod download: %w", err)
	}

	// Copy go.sum to deps dir as manifest
	goSumPath := filepath.Join(workDir, "go.sum")
	if data, err := os.ReadFile(goSumPath); err == nil {
		_ = os.WriteFile(filepath.Join(targetDir, "go.sum"), data, 0644)
	}

	return nil
}

// ─── Python deps ──────────────────────────────────────────────────────

func installPythonDeps(workDir string) error {
	reqPath := filepath.Join(workDir, "requirements.txt")
	if _, err := os.Stat(reqPath); os.IsNotExist(err) {
		return fmt.Errorf("no requirements.txt found in %s", workDir)
	}

	targetDir := filepath.Join(depsDir, "python")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create deps dir: %w", err)
	}

	// Create virtual environment if missing
	venvPath := filepath.Join(targetDir, "venv")
	if _, err := os.Stat(filepath.Join(venvPath, "bin", "python3")); os.IsNotExist(err) {
		cmd := exec.Command("python3", "-m", "venv", venvPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create venv: %w", err)
		}
	}

	// Install via pip in the virtual env
	pipPath := filepath.Join(venvPath, "bin", "pip")
	cmd := exec.Command(pipPath, "install", "-r", reqPath, "--quiet")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pip install: %w", err)
	}

	return nil
}
