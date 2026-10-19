package config

import (
	"encoding/json"
	"strings"

	"sigs.k8s.io/yaml"
)

// FileSections reads YAML (including JSON) without normalizing user-defined map
// keys. Keep sections as raw JSON so ParseConfig uses the API's JSON unmarshallers.
func FileSections(data []byte) (map[string]json.RawMessage, error) {
	data, err := yaml.YAMLToJSONStrict(data)
	if err != nil {
		return nil, err
	}
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(data, &sections); err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(sections))
	for key, value := range sections {
		out[strings.ToLower(key)] = value
	}
	return out, nil
}

// ParseDynamicConfig is shared by initial loading and ConfigMap watch updates.
// YAML conversion preserves JSON tags and custom API unmarshallers.
func ParseDynamicConfig(data []byte) (HotReloadStruct, error) {
	var out HotReloadStruct
	err := yaml.Unmarshal(data, &out)
	return out, err
}
