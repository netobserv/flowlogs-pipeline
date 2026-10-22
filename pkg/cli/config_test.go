package cli

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestApplyConfig(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		flagType      string
		defaultValue  string
		explicitValue string
		envName       string
		envValue      string
		sections      map[string]json.RawMessage
		wantString    string
		wantSlice     []string
		wantChanged   bool
	}{
		{
			name:          "explicit flag overrides environment and file",
			flagName:      "setting",
			defaultValue:  "default",
			explicitValue: "flag",
			envName:       "TEST_SETTING",
			envValue:      "environment",
			sections:      map[string]json.RawMessage{"setting": json.RawMessage(`"file"`)},
			wantString:    "flag",
			wantChanged:   true,
		},
		{
			name:         "environment overrides file",
			flagName:     "setting",
			defaultValue: "default",
			envName:      "TEST_SETTING",
			envValue:     "environment",
			sections:     map[string]json.RawMessage{"setting": json.RawMessage(`"file"`)},
			wantString:   "environment",
			wantChanged:  true,
		},
		{
			name:         "null leaves flag unset",
			flagName:     "setting",
			defaultValue: "default",
			sections:     map[string]json.RawMessage{"setting": json.RawMessage(`null`)},
			wantString:   "default",
		},
		{
			name:         "top-level section lookup requires exact case",
			flagName:     "setting",
			defaultValue: "default",
			sections:     map[string]json.RawMessage{"Setting": json.RawMessage(`"file"`)},
			wantString:   "default",
		},
		{
			name:         "nested section lookup requires exact case",
			flagName:     "k8scache.port",
			defaultValue: "0",
			sections: map[string]json.RawMessage{
				"k8scache": json.RawMessage(`{"port": 6384}`),
			},
			wantString:  "6384",
			wantChanged: true,
		},
		{
			name:         "nested section lookup does not normalize key case",
			flagName:     "k8scache.port",
			defaultValue: "0",
			sections: map[string]json.RawMessage{
				"k8scache": json.RawMessage(`{"PoRt": 6384}`),
			},
			wantString: "0",
		},
		{
			name:        "string slice accepts JSON array",
			flagName:    "names",
			flagType:    "stringSlice",
			sections:    map[string]json.RawMessage{"names": json.RawMessage(`["first", "second"]`)},
			wantSlice:   []string{"first", "second"},
			wantChanged: true,
		},
		{
			name:         "string flag accepts object value",
			flagName:     "payload",
			defaultValue: "",
			sections:     map[string]json.RawMessage{"payload": json.RawMessage(`{"UserKey":"value"}`)},
			wantString:   `{"UserKey":"value"}`,
			wantChanged:  true,
		},
		{
			name:         "string flag accepts array value",
			flagName:     "payload",
			defaultValue: "",
			sections:     map[string]json.RawMessage{"payload": json.RawMessage(`["item", 2]`)},
			wantString:   `["item", 2]`,
			wantChanged:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			if tt.flagType == "stringSlice" {
				cmd.Flags().StringSlice(tt.flagName, nil, "")
			} else {
				cmd.Flags().String(tt.flagName, tt.defaultValue, "")
			}

			if tt.envName != "" {
				t.Setenv(tt.envName, tt.envValue)
			}
			if tt.explicitValue != "" {
				if err := cmd.Flags().Set(tt.flagName, tt.explicitValue); err != nil {
					t.Fatalf("set explicit flag: %v", err)
				}
			}

			err := ApplyConfig(cmd, tt.sections, func(flagName string) string {
				if flagName == tt.flagName {
					return tt.envName
				}
				return ""
			})
			if err != nil {
				t.Fatalf("ApplyConfig returned error: %v", err)
			}

			flag := cmd.Flags().Lookup(tt.flagName)
			if flag.Changed != tt.wantChanged {
				t.Errorf("flag Changed = %t, want %t", flag.Changed, tt.wantChanged)
			}
			if tt.flagType == "stringSlice" {
				got, err := cmd.Flags().GetStringSlice(tt.flagName)
				if err != nil {
					t.Fatalf("get string slice: %v", err)
				}
				if !reflect.DeepEqual(got, tt.wantSlice) {
					t.Errorf("flag value = %#v, want %#v", got, tt.wantSlice)
				}
				return
			}

			got, err := cmd.Flags().GetString(tt.flagName)
			if err != nil {
				t.Fatalf("get string flag: %v", err)
			}
			if got != tt.wantString {
				t.Errorf("flag value = %q, want %q", got, tt.wantString)
			}
		})
	}
}
