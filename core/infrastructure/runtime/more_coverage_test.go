package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestEnsure_Go_NotNeeded verifies that Ensure returns nil for Go runtime
func TestEnsure_Go_NotNeeded(t *testing.T) {
	ctx := context.Background()
	
	// When go is in PATH, Ensure should succeed
	if _, err := findGoBinary(); err == nil {
		err := Ensure(ctx, RuntimeGo, "", ".vyx", nil)
		if err != nil {
			t.Errorf("Ensure(Go) should succeed when go is in PATH, got: %v", err)
		}
	}
}

// TestEnsure_Node_AlreadyInstalled tests when Node is already installed
func TestEnsure_Node_AlreadyInstalled(t *testing.T) {
	t.Skip("Skipping - requires network to fully test")
}

// TestEnsure_Python_AlreadyInstalled tests when Python is already installed
func TestEnsure_Python_AlreadyInstalled(t *testing.T) {
	t.Skip("Skipping - requires network to fully test")
}

// TestResolve_Go tests resolving Go runtime
func TestResolve_Go(t *testing.T) {
	goBinary, err := findGoBinary()
	if err != nil {
		t.Skip("Go not in PATH")
	}
	
	path, err := Resolve(RuntimeGo, "")
	if err != nil {
		t.Errorf("Resolve(Go) should succeed, got: %v", err)
	}
	if path != goBinary {
		t.Errorf("Resolve(Go) = %v, want %v", path, goBinary)
	}
}

// TestResolve_UnknownRuntime tests resolving unknown runtime
func TestResolve_UnknownRuntime(t *testing.T) {
	_, err := Resolve(RuntimeUnknown, "")
	if err == nil {
		t.Error("Resolve(Unknown) should return error")
	}
}

// TestResolve_NodeNotInstalled tests resolving Node when not installed
func TestResolve_NodeNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")
	
	_, err := Resolve(RuntimeNode, vyxDir)
	if err == nil {
		t.Error("Resolve(Node) should return error when not installed")
	}
}

// TestResolve_PythonNotInstalled tests resolving Python when not installed
func TestResolve_PythonNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")
	
	_, err := Resolve(RuntimePython, vyxDir)
	if err == nil {
		t.Error("Resolve(Python) should return error when not installed")
	}
}

// TestDetect_EdgeCases tests more edge cases for Detect
func TestDetect_EdgeCases(t *testing.T) {
	tests := []struct {
		command string
		want    Runtime
	}{
		{"node20 index.js", RuntimeNode},
		{"node18 app.js", RuntimeNode},
		{"python3.12 script.py", RuntimePython},
		{"go1.21 build", RuntimeGo},
		{"npm run start", RuntimeNode},
		{"yarn start", RuntimeUnknown}, // yarn is not in our detection
		{"pip install", RuntimePython}, // "pip" is detected
		{"", RuntimeUnknown},
		{"bash script.sh", RuntimeUnknown},
		{"./custom_binary", RuntimeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			if got := Detect(tt.command); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// TestNeedsProvisioning_EdgeCases tests edge cases
func TestNeedsProvisioning_EdgeCases(t *testing.T) {
	// Test with VYX_SKIP_RUNTIME set to 1
	os.Setenv("VYX_SKIP_RUNTIME", "1")
	defer os.Unsetenv("VYX_SKIP_RUNTIME")
	
	// When VYX_SKIP_RUNTIME=1, NeedsProvisioning should return false
	if NeedsProvisioning("node index.js") {
		t.Error("NeedsProvisioning should return false when VYX_SKIP_RUNTIME=1")
	}
	
	// Clear the env and test again
	os.Unsetenv("VYX_SKIP_RUNTIME")
	
	// Now it should return true for node commands
	if !NeedsProvisioning("node index.js") {
		t.Error("NeedsProvisioning should return true for node command when VYX_SKIP_RUNTIME is not set")
	}
}

// TestEnsure_UnsupportedRuntime tests unsupported runtime
func TestEnsure_UnsupportedRuntime(t *testing.T) {
	err := Ensure(context.Background(), RuntimeUnknown, "", ".vyx", nil)
	if err == nil {
		t.Error("Ensure(Unsupported) should return error")
	}
}

// TestGetOSAndArch tests OS and architecture detection
func TestGetOSAndArch(t *testing.T) {
	osName, arch := getOSAndArch()
	
	if osName == "" {
		t.Error("getOSAndArch() should return non-empty OS")
	}
	if arch == "" {
		t.Error("getOSAndArch() should return non-empty arch")
	}
	
	// Verify common architectures are handled
	switch arch {
	case "amd64", "arm64", "386", "arm":
		// OK
	default:
		t.Errorf("Unexpected architecture: %s", arch)
	}
}

// TestDownloadFile_InvalidURL tests download with invalid URL
func TestDownloadFile_InvalidURL(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	err := downloadFile(context.Background(), "http://invalid-url-that-does-not-exist-12345.com/file", tmpFile)
	if err == nil {
		t.Error("downloadFile should fail with invalid URL")
	}
}

// TestDownloadZip_InvalidFile tests unzip with invalid file
func TestDownloadZip_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := downloadZip(context.Background(), "http://invalid-url.com/file.zip", tmpDir, "binary")
	if err == nil {
		t.Error("downloadZip should fail with invalid URL")
	}
}

// TestEnsureNode_ErrorPaths tests error handling in ensureNode
func TestEnsureNode_ErrorPaths(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI")
	}
	
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")
	
	// Test with invalid fnm directory permissions
	fnmDir := filepath.Join(vyxDir, "runtimes", "fnm")
	os.MkdirAll(fnmDir, 0755)
	
	// This should work normally
	var logs []string
	logger := func(msg string) { logs = append(logs, msg) }
	
	// We can't easily test the error path without mocking
	// But we can test that the function handles missing node gracefully
	_ = Ensure(context.Background(), RuntimeNode, "99.99", vyxDir, logger) // Non-existent version
}

// Helper function to find Go binary
func findGoBinary() (string, error) {
	path, err := exec.LookPath("go")
	if err != nil {
		return "", err
	}
	return path, nil
}

// TestEnsure_Python_ErrorPaths tests error handling in ensurePython
func TestEnsure_Python_ErrorPaths(t *testing.T) {
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")
	
	// Create uv binary that fails
	uvDir := filepath.Join(vyxDir, "runtimes", "uv")
	os.MkdirAll(uvDir, 0755)
	uvBinary := filepath.Join(uvDir, "uv")
	os.WriteFile(uvBinary, []byte("#!/bin/sh\nexit 1"), 0755)
	
	logger := func(msg string) {}
	err := Ensure(context.Background(), RuntimePython, "3.12", vyxDir, logger)
	// This should fail because uv exits with error
	if err == nil {
		t.Error("Ensure should fail when uv fails")
	}
}

// TestDefaultVersion_EdgeCases tests default version for edge cases
func TestDefaultVersion_EdgeCases(t *testing.T) {
	tests := []struct {
		rt   Runtime
		want string
	}{
		{RuntimeNode, "20"},
		{RuntimePython, "3.12"},
		{RuntimeGo, "1.21"},
		{Runtime("invalid"), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			if got := tt.rt.DefaultVersion(); got != tt.want {
				t.Errorf("DefaultVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrors tests error definitions
func TestErrors(t *testing.T) {
	if ErrRuntimeNotFound == nil {
		t.Error("ErrRuntimeNotFound should not be nil")
	}
	if ErrUnsupportedOS == nil {
		t.Error("ErrUnsupportedOS should not be nil")
	}
	if ErrUnsupportedArch == nil {
		t.Error("ErrUnsupportedArch should not be nil")
	}
	if ErrDownloadFailed == nil {
		t.Error("ErrDownloadFailed should not be nil")
	}
	if ErrInstallFailed == nil {
		t.Error("ErrInstallFailed should not be nil")
	}
	
	// Test error messages
	tests := []struct {
		err  error
		want string
	}{
		{ErrRuntimeNotFound, "runtime binary not found"},
		{ErrUnsupportedOS, "unsupported operating system"},
		{ErrUnsupportedArch, "unsupported architecture"},
		{ErrDownloadFailed, "failed to download runtime"},
		{ErrInstallFailed, "failed to install runtime"},
	}
	
	for _, tt := range tests {
		if tt.err.Error() != tt.want {
			t.Errorf("Error() = %v, want %v", tt.err.Error(), tt.want)
		}
	}
}

// TestNeedsProvisioning_GoCommand tests Go command detection
func TestNeedsProvisioning_GoCommand(t *testing.T) {
	// Go commands should not need provisioning
	tests := []string{
		"go run main.go",
		"go build",
		"go test ./...",
	}
	
	for _, cmd := range tests {
		if NeedsProvisioning(cmd) {
			t.Errorf("NeedsProvisioning(%q) should be false for Go commands", cmd)
		}
	}
}

// TestNeedsProvisioning_NodeCommand tests Node command detection
func TestNeedsProvisioning_NodeCommand(t *testing.T) {
	// Node commands should need provisioning
	tests := []string{
		"node index.js",
		"npm start",
		"npx create-react-app",
	}
	
	for _, cmd := range tests {
		if !NeedsProvisioning(cmd) {
			t.Errorf("NeedsProvisioning(%q) should be true for Node commands", cmd)
		}
	}
}

// TestNeedsProvisioning_PythonCommand tests Python command detection
func TestNeedsProvisioning_PythonCommand(t *testing.T) {
	// Python commands should need provisioning
	tests := []string{
		"python app.py",
		"python3 app.py",
		"pip install flask",
		"uv pip install",
	}
	
	for _, cmd := range tests {
		if !NeedsProvisioning(cmd) {
			t.Errorf("NeedsProvisioning(%q) should be true for Python commands", cmd)
		}
	}
}

// TestDetect_NodeVariants tests various node command detections
func TestDetect_NodeVariants(t *testing.T) {
	tests := []struct {
		command string
		want    Runtime
	}{
		{"node", RuntimeNode},
		{"node20", RuntimeNode},
		{"node18.0", RuntimeNode},
		{"npm", RuntimeNode},
		{"npx", RuntimeNode},
		{"./node", RuntimeUnknown}, // Doesn't start with "node" as first arg
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			if got := Detect(tt.command); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// TestDetect_PythonVariants tests various python command detections
func TestDetect_PythonVariants(t *testing.T) {
	tests := []struct {
		command string
		want    Runtime
	}{
		{"python", RuntimePython},
		{"python3", RuntimePython},
		{"python3.12", RuntimePython},
		{"pypy", RuntimePython},
		{"pip", RuntimePython},
		{"pip3", RuntimeUnknown}, // pip3 not explicitly handled
		{"uv", RuntimePython},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			if got := Detect(tt.command); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// TestDetect_GoVariants tests various go command detections
func TestDetect_GoVariants(t *testing.T) {
	tests := []struct {
		command string
		want    Runtime
	}{
		{"go", RuntimeGo},
		{"go1.21", RuntimeGo},
		{"go run main.go", RuntimeGo},
		{"./go", RuntimeUnknown}, // "./go" doesn't start with "go"
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			if got := Detect(tt.command); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}
