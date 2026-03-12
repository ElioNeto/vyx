package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// SchemaValidator implements application/gateway.SchemaValidator using
// santhosh-tekuri/jsonschema. Schemas are pre-compiled at startup (WarmUp)
// and cached for the lifetime of the process. InvalidateCache resets the
// cache for hot reload (vyx dev, SIGHUP).
type SchemaValidator struct {
	schemasDir string

	mu     sync.RWMutex
	cached map[string]*jsonschema.Schema
}

// NewSchemaValidator creates a validator that loads schemas from dir.
func NewSchemaValidator(schemasDir string) *SchemaValidator {
	return &SchemaValidator{
		schemasDir: schemasDir,
		cached:     make(map[string]*jsonschema.Schema),
	}
}

// WarmUp pre-compiles every *.json file in schemasDir and stores the result
// in the cache.
func (v *SchemaValidator) WarmUp() error {
	if v.schemasDir == "" {
		return nil
	}
	entries, err := os.ReadDir(v.schemasDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("schema warm-up: read dir %s: %w", v.schemasDir, err)
	}

	var errs []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		if _, err := v.compile(name); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("schema warm-up errors:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// InvalidateCache drops all cached schemas.
func (v *SchemaValidator) InvalidateCache() {
	v.mu.Lock()
	v.cached = make(map[string]*jsonschema.Schema)
	v.mu.Unlock()
}

// Validate validates body against the JSON Schema named schemaName.
func (v *SchemaValidator) Validate(schemaName string, body []byte) error {
	schema, err := v.getSchema(schemaName)
	if err != nil {
		return err
	}

	// Decode body into a generic interface{} for validation.
	var inst interface{}
	if err := json.Unmarshal(body, &inst); err != nil {
		return fmt.Errorf("schema: unmarshal body: %w", err)
	}

	if err := schema.Validate(inst); err != nil {
		return toValidationError(err)
	}
	return nil
}

// getSchema retrieves a compiled schema from cache, compiling it on first access.
func (v *SchemaValidator) getSchema(name string) (*jsonschema.Schema, error) {
	v.mu.RLock()
	s, ok := v.cached[name]
	v.mu.RUnlock()
	if ok {
		return s, nil
	}
	return v.compile(name)
}

// compile reads, compiles, and caches the schema for name.
func (v *SchemaValidator) compile(name string) (*jsonschema.Schema, error) {
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

// toValidationError converts a jsonschema validation error into a structured
// *dgw.ValidationError with per-field detail entries.
func toValidationError(err error) *dgw.ValidationError {
	ve := &dgw.ValidationError{}

	var jsErr *jsonschema.ValidationError
	if ok := asValidationError(err, &jsErr); ok {
		for _, cause := range jsErr.Causes {
			ve.Details = append(ve.Details, dgw.ValidationDetail{
				Field:   cause.InstanceLocation,
				Message: cause.Message,
			})
		}
		if len(ve.Details) == 0 {
			ve.Details = []dgw.ValidationDetail{{
				Field:   jsErr.InstanceLocation,
				Message: jsErr.Message,
			}}
		}
	} else {
		ve.Details = []dgw.ValidationDetail{{Field: "", Message: err.Error()}}
	}

	return ve
}

// asValidationError performs a type assertion to *jsonschema.ValidationError.
func asValidationError(err error, target **jsonschema.ValidationError) bool {
	if e, ok := err.(*jsonschema.ValidationError); ok {
		*target = e
		return true
	}
	return false
}

// schemaWarmUpSummary returns a JSON-serialisable summary for logging.
func schemaWarmUpSummary(count int) string {
	b, _ := json.Marshal(map[string]int{"schemas_compiled": count})
	return string(b)
}
