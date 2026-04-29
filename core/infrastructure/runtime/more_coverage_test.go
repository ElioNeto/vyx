package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestResolve_Node tests Resolve for Node.js
func TestResolve_Node(t *testing.T) {
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")

	// Create a fake fnm and node binary
	fnmDir := filepath.Join(vyxDir, "runtimes", "fnm")
	if err := os.MkdirAll(fnmDir, 0755); err != nil {
		t.Fatal(err)
	}
	fnmPath := filepath.Join(fnmDir, "fnm")
	if err := os.WriteFile(fnmPath, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatal(err)
	}

	nodeDir := filepath.Join(vyxDir, "runtimes", "node", "v20", "bin")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(nodeDir, "node")
	if err := os.WriteFile(nodePath, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatal(err)
	}

	// Resolve should find the node binary
	path, err := Resolve(RuntimeNode, vyxDir)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if path == "" {
		t.Fatal("Resolve() returned empty path")
	}
	t.Logf("Resolved node path: %s", path)
}

// TestResolve_NotFound tests Resolve when runtime not installed
func TestResolve_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")

	// Don't create any runtimes
	_, err := Resolve(RuntimeNode, vyxDir)
	if err == nil {
		t.Fatal("expected error for non-existent runtime")
	}
}

// TestGetOSAndArch_UnsupportedOS tests unsupported OS
func TestGetOSAndArch_UnsupportedOS(t *testing.T) {
	// We can't easily test this because runtime.GOOS is a constant
	// Skip this test
	t.Skip("can't test unsupported OS in test environment")
}

// TestDownloadFile_InvalidURL tests download with invalid URL
func TestDownloadFile_InvalidURL(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.bin")

	err := downloadFile(context.Background(), "http://invalid-url-12345.com/file", tmpFile)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// TestDownloadZip_InvalidFile tests unzipping invalid file
func TestDownloadZip_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "invalid.zip")

	// Create an invalid zip file
	if err := os.WriteFile(zipPath, []byte("not a zip file"), 0644); err != nil {
		t.Fatal(err)
	}

	err := downloadZip(context.Background(), zipPath, tmpDir, "binary")
	if err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

// TestEnsureGo_NotNeeded tests that Go doesn't need provisioning
func TestEnsureGo_NotNeeded(t *testing.T) {
	// Go runtime returns false for NeedsProvisioning
	// So Ensure won't be called for Go in production
	// This is tested indirectly via SpawnWorker tests
	t.Skip("Go provisioning not needed, tested via lifecycle tests")
}

// TestNeedsProvisioning_Go tests that Go doesn't need provisioning
func TestNeedsProvisioning_Go(t *testing.T) {
	if NeedsProvisioning("go run main.go") != false {
		t.Error("expected false for go command")
	}
	if NeedsProvisioning("go build") != false {
		t.Error("expected false for go command")
	}
}
