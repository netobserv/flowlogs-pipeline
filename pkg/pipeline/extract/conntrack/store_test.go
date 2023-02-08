/*
 * Copyright (C) 2023 IBM, Inc.
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

package conntrack

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestSchedulingGroupToLabelValue(t *testing.T) {
	table := []struct {
		name     string
		groupIdx int
		group    api.ConnTrackSchedulingGroup
		expected string
	}{
		{
			"Non-default scheduling group",
			0,
			api.ConnTrackSchedulingGroup{
				Selector: map[string]interface{}{
					"Proto": 1,
					"ip":    "10.0.0.0",
				},
			},
			"0: Proto=1, ip=10.0.0.0, ",
		},
		{
			"Default scheduling group",
			0,
			api.ConnTrackSchedulingGroup{},
			"0: DEFAULT",
		},
	}
	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			actual := schedulingGroupToLabelValue(test.groupIdx, test.group)
			require.Equal(t, test.expected, actual)
		})
	}
}
