package jsonkeys

import (
	"fmt"

	"github.com/bytedance/sonic"
)

func ToCamelCaseJSON(data []byte) ([]byte, error) {
	var value any
	if err := sonic.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	transformed := transformTree(value, SnakeToCamel)
	return sonic.Marshal(transformed)
}

func FromCamelCaseJSON(data []byte) ([]byte, error) {
	var value any
	if err := sonic.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	transformed := transformTree(value, CamelToSnake)
	return sonic.Marshal(transformed)
}
