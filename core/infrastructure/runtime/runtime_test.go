package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		command string
		want    Runtime
	}{
		{"node backend/index.js", RuntimeNode},
		{"python backend/serve.py", RuntimePython},
		{"python3 backend/serve.py", RuntimePython},
		{"pypy backend/serve.py", RuntimePython},
		{"go run ./backend/main.go", RuntimeGo},
		{"go run backend/server.go", RuntimeGo},
		{"npm start", RuntimeNode},
		{"npx serve", RuntimeNode},
		{"pip install -r requirements.txt", RuntimePython},
		{"uv pip install -r requirements.txt", RuntimePython},
		{"", RuntimeUnknown},
		{"./bin/myworker", RuntimeUnknown},
		{"bash script.sh", RuntimeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			if got := Detect(tt.command); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestEnsure_Node_DownloadFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	original := fnmDownloadBaseURL
	fnmDownloadBaseURL = ts.URL
	defer func() { fnmDownloadBaseURL = original }()

	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")

	err := Ensure(context.Background(), RuntimeNode, "20", vyxDir, nil)
	if err == nil {
		t.Error("Ensure() should return error when download returns 404")
	}
}

func TestEnsure_Node_DownloadsMockFNM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		fw, _ := zw.Create("fnm")
		fw.Write([]byte("#!/bin/sh\necho fake fnm\n"))
		zw.Close()
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer ts.Close()

	original := fnmDownloadBaseURL
	fnmDownloadBaseURL = ts.URL
	defer func() { fnmDownloadBaseURL = original }()

	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")

	var logs []string
	logger := func(msg string) { logs = append(logs, msg) }

	_ = Ensure(context.Background(), RuntimeNode, "20", vyxDir, logger)

	fnmPath := filepath.Join(vyxDir, "runtimes", "fnm", "fnm")
	if _, statErr := os.Stat(fnmPath); statErr != nil {
		t.Errorf("fnm binary should exist after download, got stat error: %v", statErr)
	}
}

func TestEnsure_Python_DownloadFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	original := uvDownloadBaseURL
	uvDownloadBaseURL = ts.URL
	defer func() { uvDownloadBaseURL = original }()

	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")

	err := Ensure(context.Background(), RuntimePython, "3.12", vyxDir, nil)
	if err == nil {
		t.Error("Ensure() should return error when download returns 404")
	}
}

func TestEnsure_Python_DownloadsMockUV(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("#!/bin/sh\necho fake uv\n"))
	}))
	defer ts.Close()

	original := uvDownloadBaseURL
	uvDownloadBaseURL = ts.URL
	defer func() { uvDownloadBaseURL = original }()

	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")

	var logs []string
	logger := func(msg string) { logs = append(logs, msg) }

	_ = Ensure(context.Background(), RuntimePython, "3.12", vyxDir, logger)

	uvPath := filepath.Join(vyxDir, "runtimes", "uv", "uv")
	if _, statErr := os.Stat(uvPath); statErr != nil {
		t.Errorf("uv binary should exist after download, got stat error: %v", statErr)
	}
}

func TestEnsure_Go_NotInPath(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")

	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)
	os.Setenv("PATH", tmpDir)

	err := Ensure(ctx, RuntimeGo, "", vyxDir, nil)
	if err == nil {
		t.Error("Ensure() should return error when Go is not in PATH")
	}
}

func TestResolve_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	vyxDir := filepath.Join(tmpDir, ".vyx")

	_, err := Resolve(RuntimeNode, vyxDir)
	if err == nil {
		t.Error("Resolve() should return error when runtime not installed")
	}
}

func TestRuntime_DefaultVersion(t *testing.T) {
	tests := []struct {
		rt   Runtime
		want string
	}{
		{RuntimeNode, "20"},
		{RuntimePython, "3.12"},
		{RuntimeGo, "1.21"},
		{RuntimeUnknown, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			if got := tt.rt.DefaultVersion(); got != tt.want {
				t.Errorf("DefaultVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}