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
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/stretchr/testify/require"
)

func Test_dedupeNetworkTransformRules(t *testing.T) {
	slice := api.NetworkTransformRules{
		api.NetworkTransformRule{Type: "add_kubernetes", Kubernetes: &api.K8sRule{IPField: "i1", Output: "o1"}},
		api.NetworkTransformRule{Type: "add_kubernetes", Kubernetes: &api.K8sRule{IPField: "i2", Output: "o2"}},
		api.NetworkTransformRule{Type: "add_kubernetes", Kubernetes: &api.K8sRule{IPField: "i3", Output: "o3"}},
		api.NetworkTransformRule{Type: "add_kubernetes", Kubernetes: &api.K8sRule{IPField: "i2", Output: "o2"}},
	}
	expected := api.NetworkTransformRules{
		api.NetworkTransformRule{Type: "add_kubernetes", Kubernetes: &api.K8sRule{IPField: "i1", Output: "o1"}},
		api.NetworkTransformRule{Type: "add_kubernetes", Kubernetes: &api.K8sRule{IPField: "i2", Output: "o2"}},
		api.NetworkTransformRule{Type: "add_kubernetes", Kubernetes: &api.K8sRule{IPField: "i3", Output: "o3"}},
	}
	actual := dedupeNetworkTransformRules(slice)

	require.ElementsMatch(t, actual, expected)
}

func Test_dedupeAggregateDefinitions(t *testing.T) {
	slice := api.AggregateDefinitions{
		api.AggregateDefinition{Name: "n1", GroupByKeys: api.AggregateBy{"a", "b"}, OperationType: api.AggregateOperation("o1")},
		api.AggregateDefinition{Name: "n1", GroupByKeys: api.AggregateBy{"a"}, OperationType: api.AggregateOperation("o1")},
		api.AggregateDefinition{Name: "n2", GroupByKeys: api.AggregateBy{"a", "b"}, OperationType: api.AggregateOperation("o2")},
		api.AggregateDefinition{Name: "n3", GroupByKeys: api.AggregateBy{"a", "b"}, OperationType: api.AggregateOperation("o3")},
		api.AggregateDefinition{Name: "n2", GroupByKeys: api.AggregateBy{"a", "b"}, OperationType: api.AggregateOperation("o2")},
	}
	expected := api.AggregateDefinitions{
		api.AggregateDefinition{Name: "n1", GroupByKeys: api.AggregateBy{"a", "b"}, OperationType: api.AggregateOperation("o1")},
		api.AggregateDefinition{Name: "n1", GroupByKeys: api.AggregateBy{"a"}, OperationType: api.AggregateOperation("o1")},
		api.AggregateDefinition{Name: "n2", GroupByKeys: api.AggregateBy{"a", "b"}, OperationType: api.AggregateOperation("o2")},
		api.AggregateDefinition{Name: "n3", GroupByKeys: api.AggregateBy{"a", "b"}, OperationType: api.AggregateOperation("o3")},
	}
	actual := dedupeAggregateDefinitions(slice)

	require.ElementsMatch(t, actual, expected)
}
