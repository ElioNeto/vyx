package codec_test

import (
	"testing"

	"github.com/ElioNeto/vyx/core/infrastructure/ipc/codec"
)

func TestArrowCodec_RoundTrip_MapSlice(t *testing.T) {
	c := codec.ArrowCodec{}

	input := []map[string]any{
		{"name": "Alice", "age": 30, "active": true},
		{"name": "Bob", "age": 25, "active": false},
		{"name": "Charlie", "age": 35, "active": true},
	}

	data, err := c.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got []map[string]any
	if err := c.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(got) != len(input) {
		t.Fatalf("got %d rows, want %d", len(got), len(input))
	}

	for i := range input {
		for k, wantVal := range input[i] {
			gotVal, ok := got[i][k]
			if !ok {
				t.Errorf("row %d: missing key %q", i, k)
				continue
			}
			if gotVal != wantVal {
				t.Errorf("row %d, key %q: want %v, got %v", i, k, wantVal, gotVal)
			}
		}
	}
}

func TestArrowCodec_RoundTrip_SingleMap(t *testing.T) {
	c := codec.ArrowCodec{}

	input := map[string]any{
		"route":  "/api/users",
		"method": "GET",
		"count":  42,
	}

	data, err := c.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got []map[string]any
	if err := c.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}

	for k, wantVal := range input {
		gotVal, ok := got[0][k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("key %q: want %v, got %v", k, wantVal, gotVal)
		}
	}
}

func TestArrowCodec_EmptySlice(t *testing.T) {
	c := codec.ArrowCodec{}

	input := []map[string]any{}

	data, err := c.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got []map[string]any
	if err := c.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got %d rows, want 0", len(got))
	}
}

func TestArrowCodec_NullValues(t *testing.T) {
	c := codec.ArrowCodec{}

	input := []map[string]any{
		{"name": "Alice", "age": nil},
		{"name": nil, "age": 30},
	}

	data, err := c.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got []map[string]any
	if err := c.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
}

func TestArrowCodec_MixedTypes(t *testing.T) {
	c := codec.ArrowCodec{}

	input := []map[string]any{
		{"name": "Alice", "score": 95.5, "tags": "admin"},
		{"name": "Bob", "score": 87.0, "tags": "user"},
	}

	data, err := c.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got []map[string]any
	if err := c.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}

	if got[0]["name"] != "Alice" || got[0]["score"] != 95.5 {
		t.Errorf("row 0: want {Alice, 95.5}, got %v", got[0])
	}
}
