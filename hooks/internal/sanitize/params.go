package sanitize

import "strings"

// sensitivePatterns lists key-name substrings (lowercase) whose values must be redacted.
var sensitivePatterns = []string{
	"password", "passwd", "pwd", "secret", "token",
	"api_key", "apikey", "auth", "bearer", "credential",
	"private_key", "access_key",
}

// RedactJSON returns a deep copy of data with sensitive values replaced by "[REDACTED]".
// A key is considered sensitive when it contains any of the sensitivePatterns (case-insensitive).
// Nil or empty input returns an empty map.
func RedactJSON(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return map[string]interface{}{}
	}
	return redactMap(data)
}

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

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
