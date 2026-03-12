package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestScaffoldProject_CreatesDirectoryTree(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	if err := scaffoldProject("my-app"); err != nil {
		t.Fatalf("scaffoldProject failed: %v", err)
	}

	expected := []string{
		"my-app/core",
		"my-app/backend/go",
		"my-app/backend/node",
		"my-app/backend/python",
		"my-app/frontend/src",
		"my-app/schemas",
		"my-app/vyx.yaml",
	}
	for _, p := range expected {
		if _, err := os.Stat(filepath.Join(tmpDir, p)); err != nil {
			t.Errorf("expected path %q to exist, got: %v", p, err)
		}
	}
}

func TestScaffoldProject_VyxYAML_IsValid(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	if err := scaffoldProject("test-project"); err != nil {
		t.Fatalf("scaffoldProject failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "test-project", "vyx.yaml"))
	if err != nil {
		t.Fatalf("vyx.yaml not found: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("vyx.yaml is not valid YAML: %v", err)
	}

	project, ok := parsed["project"].(map[string]any)
	if !ok {
		t.Fatal("vyx.yaml missing 'project' section")
	}
	if project["name"] != "test-project" {
		t.Errorf("expected project.name=test-project, got %q", project["name"])
	}
	if _, ok := parsed["workers"]; !ok {
		t.Error("vyx.yaml missing 'workers' section")
	}
	if _, ok := parsed["security"]; !ok {
		t.Error("vyx.yaml missing 'security' section")
	}
	if _, ok := parsed["ipc"]; !ok {
		t.Error("vyx.yaml missing 'ipc' section")
	}
	if _, ok := parsed["build"]; !ok {
		t.Error("vyx.yaml missing 'build' section")
	}
}

func TestScaffoldProject_AlreadyExists_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_ = os.Mkdir("existing-app", 0755)

	err := scaffoldProject("existing-app")
	if err == nil {
		t.Error("expected error when project directory already exists")
	}
}

func TestDefaultVyxYAML_ContainsProjectName(t *testing.T) {
	yamlContent := defaultVyxYAML("my-service")
	if !strings.Contains(yamlContent, "name: my-service") {
		t.Errorf("expected vyx.yaml to contain 'name: my-service', got:\n%s", yamlContent)
	}
}

func TestDefaultVyxYAML_IsValidYAML(t *testing.T) {
	yamlContent := defaultVyxYAML("valid-project")
	var parsed any
	if err := yaml.Unmarshal([]byte(yamlContent), &parsed); err != nil {
		t.Fatalf("defaultVyxYAML generated invalid YAML: %v", err)
	}
}
