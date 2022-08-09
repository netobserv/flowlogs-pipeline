/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnTrackValidate(t *testing.T) {
	// Invalid configurations
	tests := []struct {
		name   string
		config ConnTrack
	}{
		{
			"FieldGroupARef is set but FieldGroupBRef isn't",
			ConnTrack{
				KeyDefinition: KeyDefinition{Hash: ConnTrackHash{FieldGroupARef: "src"}},
			},
		},
		{
			"splitAB in non bidirectional configuration",
			ConnTrack{
				KeyDefinition: KeyDefinition{},
				OutputFields: []OutputField{
					{Name: "Bytes", Operation: "sum", SplitAB: true},
				},
			},
		},
		{
			"Unknown operation",
			ConnTrack{
				KeyDefinition: KeyDefinition{},
				OutputFields: []OutputField{
					{Name: "Bytes", Operation: "unknown"},
				},
			},
		},
		{
			"Undefined fieldGroupARef",
			ConnTrack{
				KeyDefinition: KeyDefinition{
					FieldGroups: []FieldGroup{
						{Name: "src"},
						{Name: "dst"},
					},
					Hash: ConnTrackHash{FieldGroupARef: "undefined", FieldGroupBRef: "dst"},
				},
			},
		},
		{
			"Undefined fieldGroupBRef",
			ConnTrack{
				KeyDefinition: KeyDefinition{
					FieldGroups: []FieldGroup{
						{Name: "src"},
						{Name: "dst"},
					},
					Hash: ConnTrackHash{FieldGroupARef: "src", FieldGroupBRef: "unknown"},
				},
			},
		},
		{
			"Undefined fieldGroupRefs",
			ConnTrack{
				KeyDefinition: KeyDefinition{
					FieldGroups: []FieldGroup{
						{Name: "src"},
						{Name: "dst"},
					},
					Hash: ConnTrackHash{FieldGroupRefs: []string{"unknown"}},
				},
			},
		},
		{
			"Unknown output record",
			ConnTrack{
				OutputRecordTypes: []string{"unknown"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.Error(t, err)
		})
	}
}
