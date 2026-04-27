// Package runtime provides transparent provisioning of language runtimes
// (Node.js, Python, Go) for worker execution without manual installation.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Runtime string

const (
	RuntimeNode     Runtime = "node"
	RuntimePython  Runtime = "python"
	RuntimeGo     Runtime = "go"
	RuntimeUnknown Runtime = "unknown"
)

var (
	ErrRuntimeNotFound   = errors.New("runtime binary not found")
	ErrUnsupportedOS   = errors.New("unsupported operating system")
	ErrUnsupportedArch = errors.New("unsupported architecture")
	ErrDownloadFailed  = errors.New("failed to download runtime")
	ErrInstallFailed    = errors.New("failed to install runtime")
)

var (
	fnmDownloadBaseURL = "https://github.com/Schniz/fnm/releases/latest/download"
	uvDownloadBaseURL  = "https://github.com/astral-sh/uv/releases/latest/download"
)

var defaultVersions = map[Runtime]string{
	RuntimeNode:   "20",
	RuntimePython: "3.12",
	RuntimeGo:     "1.21",
}

func Detect(command string) Runtime {
	if command == "" {
		return RuntimeUnknown
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return RuntimeUnknown
	}

	first := parts[0]

	switch first {
	case "node":
		return RuntimeNode
	case "python", "python3", "pypy":
		return RuntimePython
	case "go":
		return RuntimeGo
	case "npm", "npx":
		return RuntimeNode
	case "pip", "uv":
		return RuntimePython
	}

	if strings.HasPrefix(first, "node") {
		return RuntimeNode
	}
	if strings.HasPrefix(first, "python") {
		return RuntimePython
	}
	if strings.HasPrefix(first, "go") {
		return RuntimeGo
	}

	return RuntimeUnknown
}

func (r Runtime) DefaultVersion() string {
	return defaultVersions[r]
}

func NeedsProvisioning(command string) bool {
	if os.Getenv("VYX_SKIP_RUNTIME") == "1" {
		return false
	}

	if command == "" {
		return false
	}

	first := strings.Fields(command)[0]

	return first != "go"
}

func Ensure(ctx context.Context, rt Runtime, version, vyxDir string, logger func(string)) error {
	if vyxDir == "" {
		vyxDir = ".vyx"
	}

	targetVersion := version
	if targetVersion == "" {
		targetVersion = defaultVersions[rt]
	}

	switch rt {
	case RuntimeNode:
		return ensureNode(ctx, targetVersion, vyxDir, logger)
	case RuntimePython:
		return ensurePython(ctx, targetVersion, vyxDir, logger)
	case RuntimeGo:
		return ensureGo(ctx, logger)
	default:
		return fmt.Errorf("unsupported runtime: %s", rt)
	}
}

func ensureNode(ctx context.Context, version, vyxDir string, logger func(string)) error {
	runtimesDir := filepath.Join(vyxDir, "runtimes")
	fnmDir := filepath.Join(runtimesDir, "fnm")
	nodeDir := filepath.Join(runtimesDir, "node")

	fnmBinary := filepath.Join(fnmDir, "fnm")

	if _, err := os.Stat(fnmBinary); os.IsNotExist(err) {
		if logger != nil {
			logger("📦 Downloading fnm...")
		}
		if err := downloadFNM(ctx, fnmDir); err != nil {
			return fmt.Errorf("download fnm: %w", err)
		}
	}

	nodeBinary := filepath.Join(nodeDir, "v"+version, "bin", "node")
	if _, err := os.Stat(nodeBinary); err == nil {
		if logger != nil {
			logger(fmt.Sprintf("✅ Node.js v%s ready at %s", version, nodeBinary))
		}
		return nil
	}

	if logger != nil {
		logger(fmt.Sprintf("📦 Node.js v%s not found — installing via fnm...", version))
	}

	installCmd := exec.Command(fnmBinary, "install", version, "--fnm-dir", nodeDir)
	installCmd.Dir = runtimesDir
	if out, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fnm install: %w: %s", err, string(out))
	}

	useCmd := exec.Command(fnmBinary, "use", version, "--fnm-dir", nodeDir)
	useCmd.Dir = runtimesDir
	if out, err := useCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fnm use: %w: %s", err, string(out))
	}

	if logger != nil {
		logger(fmt.Sprintf("✅ Node.js v%s ready at %s", version, nodeBinary))
	}

	return nil
}

func ensurePython(ctx context.Context, version, vyxDir string, logger func(string)) error {
	runtimesDir := filepath.Join(vyxDir, "runtimes")
	uvDir := filepath.Join(runtimesDir, "uv")
	pythonDir := filepath.Join(runtimesDir, "python")

	uvBinary := filepath.Join(uvDir, "uv")

	if _, err := os.Stat(uvBinary); os.IsNotExist(err) {
		if logger != nil {
			logger("📦 Downloading uv...")
		}
		if err := downloadUV(ctx, uvDir); err != nil {
			return fmt.Errorf("download uv: %w", err)
		}
	}

	installCmd := exec.Command(uvBinary, "python", "install", version)
	installCmd.Dir = runtimesDir
	installCmd.Env = append(os.Environ(), "UV_PYTHON_INSTALL_DIR="+pythonDir)

	if out, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("uv python install: %w: %s", err, string(out))
	}

	matches, _ := filepath.Glob(filepath.Join(pythonDir, "cpython-"+version+"*", "bin", "python3"))
	if len(matches) == 0 {
		matches, _ = filepath.Glob(filepath.Join(pythonDir, "cpython-*", "bin", "python3"))
	}

	if len(matches) > 0 {
		if logger != nil {
			logger(fmt.Sprintf("✅ Python v%s ready at %s", version, matches[0]))
		}
		return nil
	}

	return fmt.Errorf("python %s not found after install", version)
}

func ensureGo(ctx context.Context, logger func(string)) error {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		if logger != nil {
			logger("❌ Go not found in PATH — please install Go 1.21+")
		}
		return errors.New("go not found in PATH")
	}

	if logger != nil {
		logger(fmt.Sprintf("✅ Go ready at %s", goBinary))
	}

	return nil
}

func Resolve(rt Runtime, vyxDir string) (string, error) {
	if vyxDir == "" {
		vyxDir = ".vyx"
	}

	runtimesDir := filepath.Join(vyxDir, "runtimes")

	switch rt {
	case RuntimeNode:
		matches, err := filepath.Glob(filepath.Join(runtimesDir, "node", "v*", "bin", "node"))
		if err != nil || len(matches) == 0 {
			return "", fmt.Errorf("node not installed in %s", filepath.Join(runtimesDir, "node"))
		}
		return matches[0], nil

	case RuntimePython:
		matches, err := filepath.Glob(filepath.Join(runtimesDir, "python", "cpython-*", "bin", "python3"))
		if err != nil || len(matches) == 0 {
			return "", fmt.Errorf("python not installed in %s", filepath.Join(runtimesDir, "python"))
		}
		return matches[0], nil

	case RuntimeGo:
		goBinary, err := exec.LookPath("go")
		if err != nil {
			return "", errors.New("go not found in PATH")
		}
		return goBinary, nil

	default:
		return "", fmt.Errorf("unsupported runtime: %s", rt)
	}
}

func getOSAndArch() (string, string) {
	os := runtime.GOOS
	arch := runtime.GOARCH

	if arch == "x86_64" {
		arch = "amd64"
	}
	if arch == "aarch64" {
		arch = "arm64"
	}

	return os, arch
}

func downloadFNM(ctx context.Context, targetDir string) error {
	osName, arch := getOSAndArch()

	var filename string
	if osName == "windows" {
		filename = fmt.Sprintf("fnm-%s-%s.zip", osName, arch)
	} else {
		filename = fmt.Sprintf("fnm-%s-%s.zip", osName, arch)
	}

	url := fmt.Sprintf("%s/%s", fnmDownloadBaseURL, filename)

	return downloadZip(ctx, url, targetDir, "fnm")
}

func downloadUV(ctx context.Context, targetDir string) error {
	osName, arch := getOSAndArch()

	var filename string
	if osName == "windows" {
		filename = fmt.Sprintf("uv-%s-%s.exe", arch, "pc-windows-msvc")
	} else if osName == "darwin" {
		filename = fmt.Sprintf("uv-%s-apple-darwin", arch)
	} else {
		filename = fmt.Sprintf("uv-%s-unknown-linux-%s", arch, arch)
	}

	url := fmt.Sprintf("%s/%s", uvDownloadBaseURL, filename)

	uvPath := filepath.Join(targetDir, "uv")
	if osName == "windows" {
		uvPath += ".exe"
	}

	return downloadFile(ctx, url, uvPath)
}

func downloadZip(ctx context.Context, url, targetDir, binaryName string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmpFile := filepath.Join(os.TempDir(), "vyx-"+binaryName+".zip")
	if err := downloadFile(ctx, url, tmpFile); err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	cmd := exec.Command("unzip", "-o", "-q", tmpFile, "-d", targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unzip: %w: %s", err, string(out))
	}

	binaryPath := filepath.Join(targetDir, binaryName)
	if _, err := os.Stat(binaryPath); err != nil {
		list, _ := filepath.Glob(filepath.Join(targetDir, "*", binaryName))
		if len(list) > 0 {
			binaryPath = list[0]
		} else {
			return fmt.Errorf("binary %s not found after extract", binaryName)
		}
	}

	return os.Chmod(binaryPath, 0755)
}

func downloadFile(ctx context.Context, url, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return out.Chmod(0755)
}

type Logger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
}

type DefaultLogger struct {
	w io.Writer
}

func (l *DefaultLogger) Print(v ...interface{}) {
	fmt.Fprint(l.w, v...)
}

func (l *DefaultLogger) Printf(format string, v ...interface{}) {
	fmt.Fprintf(l.w, format, v...)
}

func NewDefaultLogger(w io.Writer) *DefaultLogger {
	return &DefaultLogger{w: w}
}