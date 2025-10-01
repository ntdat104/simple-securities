package jsn

import (
	"encoding/json"
	"fmt"
)

// ToJSON marshals a Go value into a compact JSON string.
func ToJSON(v any) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(bytes), nil
}

// ToPrettyJSON marshals a Go value into a pretty-printed JSON string.
func ToPrettyJSON(v any) (string, error) {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to pretty JSON: %w", err)
	}
	return string(bytes), nil
}

// FromJSON unmarshals a JSON string into the provided destination struct or map.
func FromJSON(jsonStr string, v any) error {
	err := json.Unmarshal([]byte(jsonStr), v)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return nil
}
