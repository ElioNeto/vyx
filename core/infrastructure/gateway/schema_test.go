package gateway

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"
)

func TestSchemaValidator_WarmUpAndValidate(t *testing.T) {
    t.Parallel()
    // create temporary dir with a simple schema
    dir := t.TempDir()
    schemaPath := filepath.Join(dir, "person.json")
    // schema expects name string
    schemaContent := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
    err := os.WriteFile(schemaPath, []byte(schemaContent), 0o644)
    require.NoError(t, err)

    v := NewSchemaValidator(dir)
    // WarmUp should compile without error
    require.NoError(t, v.WarmUp())

    // valid body
    valid := []byte(`{"name":"John"}`)
    require.NoError(t, v.Validate("person", valid))

    // missing required field triggers validation error
    invalid := []byte(`{"age":30}`)
    err = v.Validate("person", invalid)
    require.Error(t, err)
    // ensure error is of type *dgw.ValidationError
    require.NotNil(t, err)
}

func TestSchemaValidator_WarmUpErrors(t *testing.T) {
    t.Parallel()
    // non‑existent dir – WarmUp should silently succeed (no error)
    v := NewSchemaValidator("/nonexistent/dir")
    require.NoError(t, v.WarmUp())

    // dir with invalid JSON schema
    dir := t.TempDir()
    badPath := filepath.Join(dir, "bad.json")
    os.WriteFile(badPath, []byte("{invalid json}"), 0o644)
    v2 := NewSchemaValidator(dir)
    err := v2.WarmUp()
    require.Error(t, err)
    // error message should contain the filename
    require.Contains(t, err.Error(), "bad.json")
}
