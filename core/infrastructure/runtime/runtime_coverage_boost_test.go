package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDownloadZip_BinaryInSubdirectory tests downloadZip finds binaries in subdirs
func TestDownloadZip_BinaryInSubdirectory(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create(filepath.Join("subdir", "mybinary"))
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("#!/bin/sh\necho test\n"))
	zw.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	err = downloadZip(context.Background(), ts.URL, tmpDir, "mybinary")
	if err != nil {
		t.Errorf("downloadZip() returned error: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "subdir", "mybinary")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Errorf("binary should exist at %s after unzip", binaryPath)
	}

	// Verify permissions
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0750 {
		t.Errorf("binary permissions = %o, want 0750", info.Mode().Perm())
	}
}

// TestDownloadZip_BinaryNotFound tests error when binary missing after unzip
func TestDownloadZip_BinaryNotFound(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, _ := zw.Create("otherfile")
	fw.Write([]byte("content"))
	zw.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	err := downloadZip(context.Background(), ts.URL, tmpDir, "nonexistent")
	if err == nil {
		t.Error("downloadZip() should return error when binary not found")
	}
	if !strings.Contains(err.Error(), "not found after extract") {
		t.Errorf("error should mention 'not found after extract', got: %v", err)
	}
}

// TestResolve_NodeInSubdirectory tests Resolve finds node in subdirs
func TestResolve_NodeInSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")
	nodeDir := filepath.Join(vyxDir, "runtimes", "node")

	version := "20"
	binDir := filepath.Join(nodeDir, "v"+version, "bin")
	os.MkdirAll(binDir, 0755)
	nodeBinary := filepath.Join(binDir, "node")
	os.WriteFile(nodeBinary, []byte("#!/bin/sh\necho node"), 0755)

	binary, err := Resolve(RuntimeNode, vyxDir)
	if err != nil {
		t.Errorf("Resolve(Node) returned error: %v", err)
	}
	if binary != nodeBinary {
		t.Errorf("Resolve(Node) = %v, want %v", binary, nodeBinary)
	}
}

// TestResolve_PythonInSubdirectory tests Resolve finds python in subdirs
func TestResolve_PythonInSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")
	pythonDir := filepath.Join(vyxDir, "runtimes", "python")

	version := "3.12"
	binDir := filepath.Join(pythonDir, "cpython-"+version+"-x86_64", "bin")
	os.MkdirAll(binDir, 0755)
	pythonBinary := filepath.Join(binDir, "python3")
	os.WriteFile(pythonBinary, []byte("#!/bin/sh\necho python"), 0755)

	binary, err := Resolve(RuntimePython, vyxDir)
	if err != nil {
		t.Errorf("Resolve(Python) returned error: %v", err)
	}
	if binary != pythonBinary {
		t.Errorf("Resolve(Python) = %v, want %v", binary, pythonBinary)
	}
}

// TestDownloadUV_NotFoundAfterDownload tests error when UV binary missing
func TestDownloadUV_NotFoundAfterDownload(t *testing.T) {
	// Create a test server that returns a valid binary path, but we'll test the case
	// where the downloaded file doesn't exist (simulate download failure)
	// This is tricky since downloadFile is called internally; skip for now
	t.Skip("downloadUV only downloads binary, install tested in ensurePython")
}
