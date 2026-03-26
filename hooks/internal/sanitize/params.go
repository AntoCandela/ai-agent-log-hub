// Package sanitize provides parameter sanitization for hook payloads.
//
// Before a hook event is sent to the Log Hub backend, all tool input and
// output data passes through RedactJSON. This ensures that secrets
// (passwords, API keys, tokens, etc.) that may appear in tool arguments or
// results are replaced with "[REDACTED]" before they ever leave the
// developer's machine.
//
// The redaction is based on key-name heuristics: if a JSON key contains a
// substring like "password", "token", or "api_key" (case-insensitive), its
// entire value is replaced regardless of depth in the JSON tree.
package sanitize

import "strings"

// sensitivePatterns lists key-name substrings (lowercase) whose values must
// be redacted. Any JSON key whose lowercased name contains one of these
// patterns will have its value replaced with "[REDACTED]".
var sensitivePatterns = []string{
	"password", "passwd", "pwd", "secret", "token",
	"api_key", "apikey", "auth", "bearer", "credential",
	"private_key", "access_key",
}

// RedactJSON returns a deep copy of data with sensitive values replaced by
// "[REDACTED]". A key is considered sensitive when it contains any of the
// sensitivePatterns (case-insensitive). Nil or empty input returns an empty
// map so callers never need nil checks.
func RedactJSON(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return map[string]interface{}{}
	}
	return redactMap(data)
}

// redactMap iterates over every key in a map, redacting sensitive keys and
// recursing into nested maps/arrays for non-sensitive keys.
func redactMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if isSensitiveKey(k) {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = redactValue(v)
	}
	return out
}

// redactValue handles recursive descent into nested maps and arrays.
// Scalar values (strings, numbers, bools) are returned as-is.
func redactValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return redactMap(val)
	case []interface{}:
		arr := make([]interface{}, len(val))
		for i, item := range val {
			arr[i] = redactValue(item)
		}
		return arr
	default:
		return v
	}
}

// isSensitiveKey checks whether a JSON key name matches any of the known
// sensitive patterns (case-insensitive substring match).
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
