package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func runDev(args []string) {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	configPath := fs.String("config", "vyx.yaml", "Path to vyx.yaml")
	addr := fs.String("addr", ":8080", "HTTP listen address")
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

	// Build the core binary first so we run the compiled version.
	fmt.Println("🔧 Building core...")
	build := exec.Command("go", "build", "-o", ".vyx/core", "./core/cmd/vyx")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: build failed: %v\n", err)
		os.Exit(1)
	}

	// Launch the core process.
	cmd := exec.Command(".vyx/core")
	cmd.Env = append(os.Environ(),
		"VYX_CONFIG="+*configPath,
		"VYX_ADDR="+*addr,
		"VYX_ENV=development",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to start core: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ vyx core running (pid %d)\n", cmd.Process.Pid)
	fmt.Println("   Press Ctrl+C to stop. Send SIGHUP to reload config.")

	// Forward SIGINT/SIGTERM to the child process for graceful shutdown.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Printf("\n🛑 received %s — stopping core...\n", sig)
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}()

	_ = cmd.Wait()
	fmt.Println("👋 vyx dev stopped")
}
