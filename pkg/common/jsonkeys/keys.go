package jsonkeys

import (
	"strings"
	"unicode"
)

// SnakeToCamel converts snake_case keys to lowerCamelCase.
func SnakeToCamel(key string) string {
	if key == "" || !strings.Contains(key, "_") {
		return key
	}
	parts := strings.Split(key, "_")
	out := parts[0]
	var outSb15 strings.Builder
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		outSb15.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	out += outSb15.String()
	return out
}

// CamelToSnake converts lowerCamelCase / PascalCase keys to snake_case.
func CamelToSnake(key string) string {
	if key == "" {
		return key
	}
	var b strings.Builder
	for i, r := range key {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func transformValue(value any, keyFn func(string) string) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[keyFn(key)] = transformValue(child, keyFn)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = transformValue(child, keyFn)
		}
		return out
	default:
		return value
	}
}

func transformTree(value any, keyFn func(string) string) any {
	return transformValue(value, keyFn)
}
