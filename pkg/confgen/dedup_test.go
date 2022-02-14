/*
 * Copyright (C) 2021 IBM, Inc.
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


package confgen

import (
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/extract/aggregate"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_dedupeNetworkTransformRules(t *testing.T) {
	slice := api.NetworkTransformRules{
		api.NetworkTransformRule{Input: "i1", Output: "o1"},
		api.NetworkTransformRule{Input: "i2", Output: "o2"},
		api.NetworkTransformRule{Input: "i3", Output: "o3"},
		api.NetworkTransformRule{Input: "i2", Output: "o2"},
	}
	expected := api.NetworkTransformRules{
		api.NetworkTransformRule{Input: "i1", Output: "o1"},
		api.NetworkTransformRule{Input: "i2", Output: "o2"},
		api.NetworkTransformRule{Input: "i3", Output: "o3"},
	}
	actual := dedupeNetworkTransformRules(slice)

	require.ElementsMatch(t, actual, expected)
}

func Test_dedupeAggregateDefinitions(t *testing.T) {
	slice := aggregate.Definitions{
		aggregate.Definition{Name: "n1", By: aggregate.By{"a", "b"}, Operation: aggregate.Operation("o1")},
		aggregate.Definition{Name: "n1", By: aggregate.By{"a"}, Operation: aggregate.Operation("o1")},
		aggregate.Definition{Name: "n2", By: aggregate.By{"a", "b"}, Operation: aggregate.Operation("o2")},
		aggregate.Definition{Name: "n3", By: aggregate.By{"a", "b"}, Operation: aggregate.Operation("o3")},
		aggregate.Definition{Name: "n2", By: aggregate.By{"a", "b"}, Operation: aggregate.Operation("o2")},
	}
	expected := aggregate.Definitions{
		aggregate.Definition{Name: "n1", By: aggregate.By{"a", "b"}, Operation: aggregate.Operation("o1")},
		aggregate.Definition{Name: "n1", By: aggregate.By{"a"}, Operation: aggregate.Operation("o1")},
		aggregate.Definition{Name: "n2", By: aggregate.By{"a", "b"}, Operation: aggregate.Operation("o2")},
		aggregate.Definition{Name: "n3", By: aggregate.By{"a", "b"}, Operation: aggregate.Operation("o3")},
	}
	actual := dedupeAggregateDefinitions(slice)

	require.ElementsMatch(t, actual, expected)
}
