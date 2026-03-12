package gateway

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// SchemaValidator implements application/gateway.SchemaValidator using
// santhosh-tekuri/jsonschema. Schemas are loaded lazily and cached.
type SchemaValidator struct {
	schemasDir string

	mu      sync.RWMutex
	cached  map[string]*jsonschema.Schema
}

// NewSchemaValidator creates a validator that loads schemas from dir.
func NewSchemaValidator(schemasDir string) *SchemaValidator {
	return &SchemaValidator{
		schemasDir: schemasDir,
		cached:     make(map[string]*jsonschema.Schema),
	}
}

// Validate validates body against the JSON Schema named schemaName.
// schemaName maps to <schemasDir>/<schemaName>.json.
func (v *SchemaValidator) Validate(schemaName string, body []byte) error {
	schema, err := v.getSchema(schemaName)
	if err != nil {
		return err
	}

	var inst any
	if err := jsonschema.UnmarshalJSON(bytes.NewReader(body), &inst); err != nil {
		return fmt.Errorf("schema: unmarshal body: %w", err)
	}

	if err := schema.Validate(inst); err != nil {
		return err
	}
	return nil
}

func (v *SchemaValidator) getSchema(name string) (*jsonschema.Schema, error) {
	v.mu.RLock()
	s, ok := v.cached[name]
	v.mu.RUnlock()
	if ok {
		return s, nil
	}

	path := filepath.Join(v.schemasDir, name+".json")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("schema: file not found for %q: %w", name, err)
	}

	compiler := jsonschema.NewCompiler()
	s, err := compiler.Compile(path)
	if err != nil {
		return nil, fmt.Errorf("schema: compile %s: %w", path, err)
	}

	v.mu.Lock()
	v.cached[name] = s
	v.mu.Unlock()
	return s, nil
}
