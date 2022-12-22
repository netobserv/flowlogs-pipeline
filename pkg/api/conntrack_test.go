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
		name        string
		config      ConnTrack
		expectedErr conntrackInvalidError
	}{
		{
			"FieldGroupARef is set but FieldGroupBRef isn't",
			ConnTrack{
				KeyDefinition: KeyDefinition{Hash: ConnTrackHash{FieldGroupARef: "src"}},
			},
			conntrackInvalidError{fieldGroupABOnlyOneIsSet: true},
		},
		{
			"splitAB in non bidirectional configuration",
			ConnTrack{
				KeyDefinition: KeyDefinition{},
				OutputFields: []OutputField{
					{Name: "Bytes", Operation: "sum", SplitAB: true},
				},
			},
			conntrackInvalidError{splitABWithNoBidi: true},
		},
		{
			"Unknown operation",
			ConnTrack{
				KeyDefinition: KeyDefinition{},
				OutputFields: []OutputField{
					{Name: "Bytes", Operation: "unknown"},
				},
			},
			conntrackInvalidError{unknownOperation: true},
		},
		{
			"Duplicate field groups",
			ConnTrack{
				KeyDefinition: KeyDefinition{
					FieldGroups: []FieldGroup{
						{Name: "src"},
						{Name: "src"},
					},
				},
			},
			conntrackInvalidError{duplicateFieldGroup: true},
		},
		{
			"Duplicate output field names (1)",
			ConnTrack{
				OutputFields: []OutputField{
					{Name: "Bytes", Operation: "min"},
					{Name: "Bytes", Operation: "max"},
				},
			},
			conntrackInvalidError{duplicateOutputFieldNames: true},
		},
		{
			"Duplicate output field names (2)",
			ConnTrack{
				KeyDefinition: KeyDefinition{
					FieldGroups: []FieldGroup{
						{Name: "src"},
						{Name: "dst"},
					},
					Hash: ConnTrackHash{
						FieldGroupARef: "src",
						FieldGroupBRef: "dst",
					},
				},
				OutputFields: []OutputField{
					{Name: "Bytes", Operation: "min", SplitAB: true},
					{Name: "Bytes_AB", Operation: "max"},
				},
			},
			conntrackInvalidError{duplicateOutputFieldNames: true},
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
			conntrackInvalidError{undefinedFieldGroupARef: true},
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
			conntrackInvalidError{undefinedFieldGroupBRef: true},
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
			conntrackInvalidError{undefinedFieldGroupRef: true},
		},
		{
			"Unknown output record",
			ConnTrack{
				OutputRecordTypes: []string{"unknown"},
			},
			conntrackInvalidError{unknownOutputRecord: true},
		},
		{
			"Undefined selector key",
			ConnTrack{
				KeyDefinition: KeyDefinition{
					FieldGroups: []FieldGroup{
						{Name: "src", Fields: []string{"srcIP"}},
					},
				},
				Scheduling: []ConnTrackSchedulingGroup{
					{
						Selector: map[string]string{
							"srcIP":         "value",
							"undefined_key": "value",
						},
					},
				},
			},
			conntrackInvalidError{undefinedSelectorKey: true},
		},
		{
			"Default selector on a scheduling group that is not the last scheduling group",
			ConnTrack{
				KeyDefinition: KeyDefinition{
					FieldGroups: []FieldGroup{
						{Name: "src", Fields: []string{"srcIP"}},
					},
				},
				Scheduling: []ConnTrackSchedulingGroup{
					{
						Selector: map[string]string{},
					},
					{
						Selector: map[string]string{
							"srcIP": "value",
						},
					},
				},
			},
			conntrackInvalidError{defaultGroupAndNotLast: true},
		},
		{
			"Exactly 1 default selector",
			ConnTrack{
				KeyDefinition: KeyDefinition{
					FieldGroups: []FieldGroup{
						{Name: "src", Fields: []string{"srcIP"}},
					},
				},
				Scheduling: []ConnTrackSchedulingGroup{},
			},
			conntrackInvalidError{exactlyOneDefaultSelector: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
