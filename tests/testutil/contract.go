package testutil

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// AssertCamelCaseKeys fails if any JSON object key contains an underscore (snake_case).
func AssertCamelCaseKeys(t *testing.T, data []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertKeysCamelCase(t, v, "$")
}

func assertKeysCamelCase(t *testing.T, v any, path string) {
	t.Helper()
	switch node := v.(type) {
	case map[string]any:
		for key, val := range node {
			if strings.Contains(key, "_") {
				t.Fatalf("snake_case key %q at %s", key, path)
			}
			assertKeysCamelCase(t, val, path+"."+key)
		}
	case []any:
		for i, item := range node {
			assertKeysCamelCase(t, item, path+"["+strconv.Itoa(i)+"]")
		}
	}
}

// AssertJSONArrayNotNull fails if a top-level JSON key is null instead of an array.
func AssertJSONArrayNotNull(t *testing.T, data []byte, keys ...string) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range keys {
		raw, ok := obj[key]
		if !ok {
			t.Fatalf("missing key %q in response: %s", key, string(data))
		}
		if string(raw) == "null" {
			t.Fatalf("key %q is JSON null, expected array", key)
		}
	}
}

func AssertHasKeys(t *testing.T, data []byte, keys ...string) {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range keys {
		if _, ok := obj[key]; !ok {
			t.Fatalf("missing key %q in response: %s", key, string(data))
		}
	}
}

// AssertNoKeys verifies sensitive keys are absent from JSON (at any depth).
func AssertNoKeys(t *testing.T, data []byte, forbidden ...string) {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range forbidden {
		if containsKey(v, key) {
			t.Fatalf("forbidden key %q found in response", key)
		}
	}
}

func containsKey(v any, target string) bool {
	switch node := v.(type) {
	case map[string]any:
		for key, val := range node {
			if key == target || containsKey(val, target) {
				return true
			}
		}
	case []any:
		for _, item := range node {
			if containsKey(item, target) {
				return true
			}
		}
	}
	return false
}
