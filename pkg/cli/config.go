package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ReadConfig reads an explicitly selected config file or searches the user's
// home directory for a YAML or JSON file with the given base name. Config
// sections are kept as raw JSON so nested user-defined keys retain their case.
func ReadConfig(fileName, defaultName string) (string, map[string]json.RawMessage, error) {
	if fileName != "" {
		sections, err := readConfigFile(fileName)
		return fileName, sections, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil, err
	}
	for _, extension := range []string{"json", "yaml", "yml"} {
		path := filepath.Join(home, defaultName+"."+extension)
		if _, err := os.Stat(path); err == nil {
			sections, readErr := readConfigFile(path)
			return path, sections, readErr
		} else if !os.IsNotExist(err) {
			return "", nil, err
		}
	}
	return "", nil, nil
}

func readConfigFile(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return config.FileSections(data)
}

// ApplyConfig applies environment and file values to flags that were not set
// explicitly. envName maps a flag name to its environment variable name.
// Environment values take precedence over config-file values.
func ApplyConfig(cmd *cobra.Command, sections map[string]json.RawMessage, envName func(string) string) error {
	var applyErr error
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if applyErr != nil || flag.Changed || flag.Name == "config" {
			return
		}

		if name := envName(flag.Name); name != "" {
			if value, found := os.LookupEnv(name); found && value != "" {
				if err := cmd.Flags().Set(flag.Name, value); err != nil {
					applyErr = fmt.Errorf("invalid environment value for %s: %w", flag.Name, err)
				}
				return
			}
		}

		raw, found, err := configValue(sections, flag.Name)
		if err != nil {
			applyErr = err
			return
		}
		if !found || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return
		}
		if flag.Value.Type() == "stringSlice" {
			var values []string
			if err := json.Unmarshal(raw, &values); err == nil {
				sliceValue, ok := flag.Value.(pflag.SliceValue)
				if !ok {
					applyErr = fmt.Errorf("flag %s does not support slice values", flag.Name)
					return
				}
				if err := sliceValue.Replace(values); err != nil {
					applyErr = fmt.Errorf("invalid config value for %s: %w", flag.Name, err)
					return
				}
				flag.Changed = true
				return
			}
		}
		value, err := flagValue(flag.Value.Type(), raw)
		if err != nil {
			applyErr = fmt.Errorf("invalid config value for %s: %w", flag.Name, err)
			return
		}
		if err := cmd.Flags().Set(flag.Name, value); err != nil {
			applyErr = fmt.Errorf("invalid config value for %s: %w", flag.Name, err)
		}
	})
	return applyErr
}

func configValue(sections map[string]json.RawMessage, flagName string) (json.RawMessage, bool, error) {
	key := strings.ToLower(flagName)
	if value, found := sections[key]; found {
		return value, true, nil
	}
	parts := strings.Split(key, ".")
	value, found := sections[parts[0]]
	if !found {
		return nil, false, nil
	}
	for _, part := range parts[1:] {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(value, &fields); err != nil {
			return nil, false, fmt.Errorf("config value for %s must be an object: %w", flagName, err)
		}
		var found bool
		value, found = caseInsensitiveValue(fields, part)
		if !found {
			return nil, false, nil
		}
	}
	return value, true, nil
}

func caseInsensitiveValue(fields map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	for field, value := range fields {
		if strings.EqualFold(field, key) {
			return value, true
		}
	}
	return nil, false
}

func flagValue(flagType string, raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if flagType == "string" && len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return string(trimmed), nil
	}
	var value interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	if str, ok := value.(string); ok {
		return str, nil
	}
	if _, ok := value.(map[string]interface{}); ok {
		encoded, err := json.Marshal(value)
		return string(encoded), err
	}
	if _, ok := value.([]interface{}); ok {
		encoded, err := json.Marshal(value)
		return string(encoded), err
	}
	return fmt.Sprint(value), nil
}
