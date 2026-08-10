/*
 * Copyright (C) 2026 IBM, Inc.
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
 */

package frr

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type staticStore struct {
	table *ASNTable
}

func (s *staticStore) Lookup(ipStr string) (uint32, bool) {
	return s.table.LookupString(ipStr)
}

func TestEnrich_WritesStringASN(t *testing.T) {
	t.Cleanup(ResetStore)
	SetStore(&staticStore{table: BuildASNTable(map[string]uint32{
		"10.128.0.0/14": 64512,
		"10.128.2.0/24": 64513,
	})})

	entry := config.GenericMap{
		"SrcAddr": "10.128.2.8",
		"DstAddr": "8.8.8.8",
	}
	Enrich(entry, &api.NetworkAddASNLabelRule{Input: "SrcAddr", Output: "SrcASN"})
	Enrich(entry, &api.NetworkAddASNLabelRule{Input: "DstAddr", Output: "DstASN"})

	require.Equal(t, "64513", entry["SrcASN"])
	_, hasDst := entry["DstASN"]
	require.False(t, hasDst)
}

func TestEnrich_NoStoreNoOp(t *testing.T) {
	t.Cleanup(ResetStore)
	ResetStore()
	entry := config.GenericMap{"SrcAddr": "10.128.2.8"}
	Enrich(entry, &api.NetworkAddASNLabelRule{Input: "SrcAddr", Output: "SrcASN"})
	_, ok := entry["SrcASN"]
	require.False(t, ok)
}

func TestInformerStore_LoadConfigs(t *testing.T) {
	s := NewInformerStore()
	err := s.LoadConfigs(&unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "a", "namespace": "ns"},
		"spec": map[string]interface{}{
			"bgp": map[string]interface{}{
				"routers": []interface{}{
					map[string]interface{}{
						"asn":      int64(64512),
						"prefixes": []interface{}{"10.128.0.0/14"},
					},
				},
			},
		},
	}})
	require.NoError(t, err)
	asn, ok := s.Lookup("10.129.0.1")
	require.True(t, ok)
	require.Equal(t, uint32(64512), asn)
}
