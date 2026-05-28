package codec_test

import (
	"fmt"
	"testing"

	"github.com/ElioNeto/vyx/core/infrastructure/ipc/codec"
)

// equal compares two values with tolerant numeric handling.
// Arrow serialises integers as int64, so we compare their string
// representations when the types differ but values are numerically equal.
func equal(got, want any) bool {
	if got == want {
		return true
	}
	// Handle numeric type mismatch (e.g. int vs int64)
	if fmt.Sprint(got) == fmt.Sprint(want) {
		return true
	}
	return false
}

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
			if !equal(gotVal, wantVal) {
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
		if !equal(gotVal, wantVal) {
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

func TestArrowCodec_UnmarshalInvalidTarget(t *testing.T) {
	c := codec.ArrowCodec{}
	input := []map[string]any{{"key": "value"}}
	data, err := c.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var wrongTarget string
	err = c.Unmarshal(data, &wrongTarget)
	if err == nil {
		t.Error("expected error when unmarshalling into non-slice target")
	}
}

func TestArrowCodec_MarshalUnsupportedType(t *testing.T) {
	c := codec.ArrowCodec{}
	_, err := c.Marshal(42)
	if err == nil {
		t.Error("expected error when marshalling unsupported type")
	}
}

func TestArrowCodec_BinaryData(t *testing.T) {
	c := codec.ArrowCodec{}
	input := []map[string]any{
		{"data": []byte{0x00, 0x01, 0x02}},
		{"data": []byte{0xFF, 0xFE}},
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

func TestArrowCodec_MixedNumericTypes(t *testing.T) {
	c := codec.ArrowCodec{}

	input := []map[string]any{
		{"int_val": int32(10), "float_val": 3.14},
		{"int_val": uint64(20), "float_val": 2.71},
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

func TestArrowCodec_BoolValues(t *testing.T) {
	c := codec.ArrowCodec{}
	input := []map[string]any{
		{"flag": true, "name": "yes"},
		{"flag": false, "name": "no"},
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
	if got[0]["flag"] != true {
		t.Errorf("row 0 flag = %v, want true", got[0]["flag"])
	}
	if got[1]["flag"] != false {
		t.Errorf("row 1 flag = %v, want false", got[1]["flag"])
	}
}

func TestArrowCodec_InlineCodec(t *testing.T) {
	var c codec.ArrowCodec
	_ = c
}
