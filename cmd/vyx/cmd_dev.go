package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ElioNeto/vyx/core/infrastructure/runtime"
	"gopkg.in/yaml.v3"
)

type workerConfig struct {
	ID              string `yaml:"id"`
	Command         string `yaml:"command"`
	WorkingDir      string `yaml:"working_dir"`
	RuntimeVersion string `yaml:"runtime_version"`
}

type projectConfig struct {
	Project map[string]any `yaml:"project"`
	Workers []workerConfig `yaml:"workers"`
}

func runDev(args []string) {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	configPath := fs.String("config", "vyx.yaml", "Path to vyx.yaml")
	addr := fs.String("addr", ":8080", "HTTP listen address")
	frontendDir := fs.String("frontend", "frontend", "Frontend directory (Vite dev server)")
	frontendPort := fs.String("frontend-port", "5173", "Frontend dev server port")
	_ = fs.Parse(args)

	if _, err := os.Stat(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: config file not found: %s\n", *configPath)
		fmt.Fprintln(os.Stderr, "Run 'vyx new project <name>' to scaffold a project first.")
		os.Exit(1)
	}

	fmt.Printf("🚀 vyx dev starting...\n")
	fmt.Printf("   config : %s\n", *configPath)
	fmt.Printf("   addr   : %s\n", *addr)
	fmt.Printf("   mode   : development (SIGHUP reloads config)\n")
	fmt.Println()

	fmt.Println("🔍 Detecting runtimes...")

	vyxDir := ".vyx"
	if d := os.Getenv("VYX_DIR"); d != "" {
		vyxDir = d
	}

	detectAndEnsureRuntimes(vyxDir)

	fmt.Println()

	// Install dependencies.
	fmt.Println("📦 Installing worker dependencies...")
	installDeps := exec.Command("vyx", "install")
	installDeps.Stdout = os.Stdout
	installDeps.Stderr = os.Stderr
	_ = installDeps.Run()

	// Build the core binary first so we run the compiled version.
	fmt.Println("🔧 Building core & scanning annotations...")

	// Force a clean build: run vyx build first to generate route_map.json,
	// infra_map.json, and compile the core binary.
	buildCore := exec.Command("vyx", "build")
	buildCore.Stdout = os.Stdout
	buildCore.Stderr = os.Stderr
	if err := buildCore.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: vyx build had errors, trying direct build: %v\n", err)
		// Fall back to direct core build.
		vyxSrc := findVyxSource()
		if vyxSrc == "" {
			fmt.Fprintln(os.Stderr, "error: could not find vyx source directory (set VYX_SRC env var)")
			os.Exit(1)
		}
		coreOut, _ := filepath.Abs(filepath.Join(vyxDir, "core"))
		fallback := exec.Command("go", "build", "-buildvcs=false", "-o", coreOut, "github.com/ElioNeto/vyx/core/cmd/vyx")
		fallback.Dir = vyxSrc
		fallback.Stdout = os.Stdout
		fallback.Stderr = os.Stderr
		if err := fallback.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: build failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Launch the core process.
	coreOut, _ := filepath.Abs(filepath.Join(vyxDir, "core"))
	cmd := exec.Command(coreOut)
	// Set up environment for the core process.
	coreEnv := append(os.Environ(),
		"VYX_CONFIG="+*configPath,
		"VYX_ADDR="+*addr,
		"VYX_ENV=development",
	)
	// Ensure JWT_SECRET is set (use default for development if not provided).
	if os.Getenv("JWT_SECRET") == "" {
		coreEnv = append(coreEnv, "JWT_SECRET=trevyx-dev-secret-at-least-32-bytes-long!!")
	}
	cmd.Env = coreEnv
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to start core: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ vyx core running (pid %d)\n", cmd.Process.Pid)

	// Start frontend dev server if directory exists.
	var frontendCmd *exec.Cmd
	if _, err := os.Stat(*frontendDir); err == nil {
		// Use centralized deps in .vyx/deps/ for frontend too
		depsNodeModules := filepath.Join(depsDir, "ts-js", "node_modules")
		frontendNM := filepath.Join(*frontendDir, "node_modules")

		if _, err := os.Stat(depsNodeModules); os.IsNotExist(err) {
			fmt.Println("📦 Installing frontend dependencies (centralized)...")
			install := exec.Command("npm", "install", "--no-package-lock", "--no-audit", "--no-fund")
			install.Dir = *frontendDir
			install.Stdout = os.Stdout
			install.Stderr = os.Stderr
			if err := install.Run(); err == nil {
				// Move to deps dir and symlink
				os.RemoveAll(depsNodeModules)
				if err := os.Rename(frontendNM, depsNodeModules); err == nil {
					rel, _ := filepath.Rel(*frontendDir, depsNodeModules)
					os.Symlink(rel, frontendNM)
				}
			}
		} else {
			// Ensure symlink exists
			if _, err := os.Lstat(frontendNM); os.IsNotExist(err) {
				rel, _ := filepath.Rel(*frontendDir, depsNodeModules)
				os.Symlink(rel, frontendNM)
			}
		}

		fmt.Println("🎨 Starting frontend dev server...")
		frontendCmd = exec.Command("npm", "run", "dev", "--", "--port", *frontendPort)
		frontendCmd.Dir = *frontendDir
		frontendCmd.Stdout = os.Stdout
		frontendCmd.Stderr = os.Stderr
		if err := frontendCmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not start frontend: %v\n", err)
		} else {
			fmt.Printf("✅ Frontend dev server running (pid %d, port %s)\n", frontendCmd.Process.Pid, *frontendPort)
		}
	}

	fmt.Println("   Press Ctrl+C to stop all processes.")

	// Forward SIGINT/SIGTERM to child processes for graceful shutdown.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Printf("\n🛑 received %s — stopping services...\n", sig)
		if frontendCmd != nil && frontendCmd.Process != nil {
			_ = frontendCmd.Process.Signal(syscall.SIGTERM)
		}
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}()

	_ = cmd.Wait()
	fmt.Println("👋 vyx dev stopped")
}

func detectAndEnsureRuntimes(vyxDir string) {
	cfg, err := loadConfig("vyx.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
		return
	}

	detected := make(map[runtime.Runtime]bool)
	for _, w := range cfg.Workers {
		rt := runtime.Detect(w.Command)
		if rt == runtime.RuntimeUnknown || detected[rt] {
			continue
		}
		detected[rt] = true

		version := w.RuntimeVersion
		if version == "" {
			version = rt.DefaultVersion()
		}

		logger := func(msg string) {
			fmt.Println(msg)
		}

		if err := runtime.Ensure(nil, rt, version, vyxDir, logger); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to ensure %s: %v\n", rt, err)
		}
	}
}

// findVyxSource locates the vyx framework source directory by checking:
//  1. VYX_SRC environment variable
//  2. Parent of the directory containing the vyx binary
//  3. Current directory (if go.mod exists for the vyx core)
func findVyxSource() string {
	if src := os.Getenv("VYX_SRC"); src != "" {
		return src
	}

	// Check if running from a vyx project with core/go.mod
	if _, err := os.Stat("core/go.mod"); err == nil {
		return "."
	}
	if _, err := os.Stat("../core/go.mod"); err == nil {
		return ".."
	}

	// Try to resolve relative to the vyx binary
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			exeDir,           // same dir as binary
			exeDir + "/..",   // parent of bin dir
			exeDir + "/../..", // grandparent
			"/mnt/data/dev/projetos/vyx",
		}
		for _, c := range candidates {
			abs, _ := filepath.Abs(c)
			if _, err := os.Stat(filepath.Join(abs, "core", "go.mod")); err == nil {
				return abs
			}
		}
	}

	return ""
}

func loadConfig(path string) (*projectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg projectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
