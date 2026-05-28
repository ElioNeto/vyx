package log

import (
	"strings"
	"testing"
)

func TestRedact_Password(t *testing.T) {
	input := `password=supersecret123`
	got := Redact(input)
	if contains(got, "supersecret123") {
		t.Errorf("Redact() did not redact password: %s", got)
	}
}

func TestRedact_JSON(t *testing.T) {
	input := `{"password":"secret123","name":"John"}`
	got := Redact(input)
	if contains(got, "secret123") {
		t.Errorf("Redact() did not redact password in JSON: %s", got)
	}
	if !contains(got, "John") {
		t.Errorf("Redact() redacted non-sensitive field")
	}
}

func TestRedact_Token(t *testing.T) {
	input := `token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0`
	got := Redact(input)
	if contains(got, "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("Redact() did not redact token")
	}
}

func TestRedact_NoSensitiveData(t *testing.T) {
	input := `name=John&email=john@example.com`
	got := Redact(input)
	if got != input {
		t.Errorf("Redact() modified input without sensitive data: %s", got)
	}
}

func TestIsSensitiveField(t *testing.T) {
	tests := []struct {
		name  string
		field string
		want  bool
	}{
		{"password exact", "password", true},
		{"password mixed case", "Password", true},
		{"token partial", "my_token", true},
		{"safe field", "username", false},
		{"safe field", "email", false},
		{"api key", "api_key", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSensitiveField(tt.field); got != tt.want {
				t.Errorf("IsSensitiveField(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 &&
		(len(substr) == 0 || (strings.Index(s, substr) >= 0)))
}
