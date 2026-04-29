package gateway

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSchemaValidator(t *testing.T) {
	v := NewSchemaValidator("/tmp/schemas")
	if v == nil {
		t.Fatal("NewSchemaValidator returned nil")
	}
	if v.schemasDir != "/tmp/schemas" {
		t.Errorf("schemasDir = %q, want %q", v.schemasDir, "/tmp/schemas")
	}
}

func TestSchemaValidator_WarmUp_EmptyDir(t *testing.T) {
	v := NewSchemaValidator("")
	err := v.WarmUp()
	if err != nil {
		t.Errorf("WarmUp with empty dir should not error, got: %v", err)
	}
}

func TestSchemaValidator_WarmUp_NonExistentDir(t *testing.T) {
	v := NewSchemaValidator("/nonexistent/path")
	err := v.WarmUp()
	if err != nil {
		t.Errorf("WarmUp with non-existent dir should not error (treated as empty): %v", err)
	}
}

func TestSchemaValidator_WarmUp_ValidSchema(t *testing.T) {
	// Create a temp dir with a valid schema
	dir := t.TempDir()
	schemaContent := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"name": { "type": "string" }
		},
		"required": ["name"]
	}`
	if err := os.WriteFile(filepath.Join(dir, "test.json"), []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	v := NewSchemaValidator(dir)
	err := v.WarmUp()
	if err != nil {
		t.Errorf("WarmUp failed: %v", err)
	}

	// Verify schema is cached by trying to validate
	err = v.Validate("test", []byte(`{"name": "John"}`))
	if err != nil {
		t.Errorf("Validate should succeed for valid data: %v", err)
	}
}

func TestSchemaValidator_Validate_InvalidSchemaName(t *testing.T) {
	v := NewSchemaValidator("/tmp")
	err := v.Validate("nonexistent", []byte(`{}`))
	if err == nil {
		t.Error("expected error for non-existent schema")
	}
}

func TestSchemaValidator_Validate_InvalidData(t *testing.T) {
	dir := t.TempDir()
	schemaContent := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"name": { "type": "string" }
		},
		"required": ["name"]
	}`
	if err := os.WriteFile(filepath.Join(dir, "user.json"), []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	v := NewSchemaValidator(dir)
	if err := v.WarmUp(); err != nil {
		t.Fatalf("WarmUp failed: %v", err)
	}

	// Missing required field "name"
	err := v.Validate("user", []byte(`{"age": 30}`))
	if err == nil {
		t.Error("expected validation error for missing required field")
	}
}

func TestSchemaValidator_InvalidateCache(t *testing.T) {
	dir := t.TempDir()
	schemaContent := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"name": { "type": "string" }
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "test.json"), []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	v := NewSchemaValidator(dir)
	v.InvalidateCache()

	// After invalidation, cache should be empty
	// We can't directly access v.cached, but we can verify behavior
	// by calling Validate which should recompile
	v.InvalidateCache() // Call again to ensure no panic
}

func TestSchemaValidator_Validate_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	schemaContent := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object"
	}`
	if err := os.WriteFile(filepath.Join(dir, "test.json"), []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	v := NewSchemaValidator(dir)
	if err := v.WarmUp(); err != nil {
		t.Fatalf("WarmUp failed: %v", err)
	}

	err := v.Validate("test", []byte(`not valid json`))
	if err == nil {
		t.Error("expected error for invalid JSON body")
	}
}

func TestSchemaValidator_Compile_InvalidSchema(t *testing.T) {
	dir := t.TempDir()
	// Write invalid JSON as schema
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`not valid json`), 0644); err != nil {
		t.Fatal(err)
	}

	v := NewSchemaValidator(dir)
	err := v.WarmUp()
	if err == nil {
		t.Error("expected error for invalid schema file")
	}
}

func TestSchemaValidator_ToValidationError(t *testing.T) {
	// Test toValidationError with a simple error
	err := &jsonschemaMockError{message: "test error", instanceLocation: "/field"}
	ve := toValidationError(err)

	if len(ve.Details) == 0 {
		t.Error("expected validation details")
	}
	if ve.Details[0].Message != "test error" {
		t.Errorf("Detail message = %q, want %q", ve.Details[0].Message, "test error")
	}
}

// Mock error type for testing
type jsonschemaMockError struct {
	message           string
	instanceLocation string
}

func (e *jsonschemaMockError) Error() string {
	return e.message
}
