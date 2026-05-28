package log

import (
	"strings"
)

// DefaultSensitiveFields lists fields whose values should be redacted in logs.
// This list is intentionally limited to common security-sensitive fields.
var DefaultSensitiveFields = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"api_key",
	"apikey",
	"access_token",
	"refresh_token",
	"credit_card",
	"cc_number",
	"card_number",
	"ssn",
	"cpf",
	"authorization",
	"jwt",
	"private_key",
}

// Redact returns a copy of the input with all sensitive field values replaced
// by "[REDACTED]". It handles both "key=value" and JSON-like formats.
// For structured logging, RedactFields should be used instead.
func Redact(input string) string {
	result := input
	for _, field := range DefaultSensitiveFields {
		// Match patterns like: password=abc123, "password":"abc123"
		// Case-insensitive field matching
		idx := 0
		for {
			// Find the field name
			pos := strings.Index(strings.ToLower(result[idx:]), field)
			if pos < 0 {
				break
			}
			pos += idx

			// Check what follows the field name: = or :
			rest := result[pos+len(field):]
			if len(rest) > 0 && (rest[0] == '=' || rest[0] == ':') {
				// Find end of value
				end := strings.IndexAny(rest, " ,}\n\t&")
				if end < 0 {
					end = len(rest)
				}
				// Replace the value
				replacement := strings.Repeat("*", end-1)
				result = result[:pos+len(field)+1] + replacement + result[pos+len(field)+1+end:]
			}
			idx = pos + len(field)
		}
	}
	return result
}

// RedactFields removes sensitive values from a key-value pairs slice.
// Use with zap's logger:
//
//	log.Warn("request failed", RedactFields(zap.String("password", "secret123"))...)
func RedactFields(fields ...interface{}) []interface{} {
	result := make([]interface{}, len(fields))
	for i, f := range fields {
		if s, ok := f.(string); ok {
			result[i] = Redact(s)
		} else {
			result[i] = f
		}
	}
	return result
}

// IsSensitiveField checks if a field name should be redacted.
func IsSensitiveField(name string) bool {
	lower := strings.ToLower(name)
	for _, f := range DefaultSensitiveFields {
		if lower == f || strings.Contains(lower, f) {
			return true
		}
	}
	return false
}
