package settings

import "gopkg.in/yaml.v3"

func toYAMLBytes(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

func fromYAMLBytes(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}
