package settings

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// toJSONMap converts a typed config section to a JSON-friendly map via YAML
// so duration strings and yaml tags work without separate json tags.
func toJSONMap(v any) (map[string]any, error) {
	y, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(y, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func fromJSONBytes(data []byte, v any) error {
	var n any
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	y, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(y, v)
}

func mapToJSON(m map[string]any) (json.RawMessage, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}
