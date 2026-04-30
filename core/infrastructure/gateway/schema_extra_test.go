package gateway

import (
    "os"
    "path/filepath"
    "testing"

    dgw "github.com/ElioNeto/vyx/core/domain/gateway"
    "github.com/stretchr/testify/require"
)

func TestSchemaValidator_ValidateBodyErrors(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    // valid schema file
    schemaPath := filepath.Join(dir, "simple.json")
    os.WriteFile(schemaPath, []byte(`{"type":"object","properties":{}}`), 0o644)
    v := NewSchemaValidator(dir)
    require.NoError(t, v.WarmUp())

    // malformed JSON body should return unmarshal error
    badBody := []byte("{invalid json")
    err := v.Validate("simple", badBody)
    require.Error(t, err)
    require.Contains(t, err.Error(), "unmarshal")
}

func TestSchemaValidator_MissingSchemaFile(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    v := NewSchemaValidator(dir)
    _, err := v.getSchema("nosuch")
    require.Error(t, err)
    require.Contains(t, err.Error(), "file not found")
}

func TestSchemaValidator_ValidationErrorDetails(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    schemaPath := filepath.Join(dir, "person.json")
    os.WriteFile(schemaPath, []byte(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`), 0o644)
    v := NewSchemaValidator(dir)
    require.NoError(t, v.WarmUp())
    body := []byte(`{"age":30}`)
    err := v.Validate("person", body)
    require.Error(t, err)
    ve, ok := err.(*dgw.ValidationError)
    require.True(t, ok)
    require.Len(t, ve.Details, 1)
    require.Contains(t, ve.Details[0].Message, "missing properties")
}
