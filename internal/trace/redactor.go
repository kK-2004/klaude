package trace

import (
	"encoding/json"
	"regexp"
	"strings"
)

var secretPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*bearer\s+|api[_-]?key\s*[:=]\s*|token\s*[:=]\s*|password\s*[:=]\s*)[^\s,;]+`)

// RedactString 抹去常见密钥形态（Bearer/api_key/token 以及 sk-/ghp_/xoxb- 前缀串）。
func RedactString(value string) string {
	value = secretPattern.ReplaceAllString(value, "$1[REDACTED]")
	for _, marker := range []string{"sk-", "ghp_", "xoxb-"} {
		if index := strings.Index(value, marker); index >= 0 {
			end := index + len(marker)
			for end < len(value) && !strings.ContainsRune(" \t\r\n,;\"'", rune(value[end])) {
				end++
			}
			value = value[:index] + marker + "[REDACTED]" + value[end:]
		}
	}
	return value
}

// RedactJSON 递归脱敏敏感键；解析失败则退回对原始字符串脱敏。
func RedactJSON(value []byte) []byte {
	var data any
	if json.Unmarshal(value, &data) != nil {
		return []byte(RedactString(string(value)))
	}
	redactValue(data)
	result, err := json.Marshal(data)
	if err != nil {
		return []byte(RedactString(string(value)))
	}
	return result
}

func redactValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if isSensitiveKey(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			redactValue(item)
		}
	case []any:
		for _, item := range typed {
			redactValue(item)
		}
	}
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, marker := range []string{"authorization", "api_key", "apikey", "token", "password", "cookie", "secret"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
