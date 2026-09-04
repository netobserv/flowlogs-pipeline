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

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractASNMappings_LocalPrefixesAndAdvertise(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "frrk8s.metallb.io/v1beta1",
		"kind":       "FRRConfiguration",
		"metadata": map[string]interface{}{
			"name":      "ovnk-generated-ra",
			"namespace": "openshift-frr-k8s",
		},
		"spec": map[string]interface{}{
			"bgp": map[string]interface{}{
				"routers": []interface{}{
					map[string]interface{}{
						"asn":      int64(64512),
						"prefixes": []interface{}{"10.128.0.0/14", "10.128.2.0/24"},
						"neighbors": []interface{}{
							map[string]interface{}{
								"address": "172.18.0.5",
								"asn":     int64(65000),
								"toAdvertise": map[string]interface{}{
									"allowed": map[string]interface{}{
										"prefixes": []interface{}{"10.128.2.0/24", "192.168.99.0/24"},
									},
								},
								"toReceive": map[string]interface{}{
									"allowed": map[string]interface{}{
										"mode":     "filtered",
										"prefixes": []interface{}{"203.0.113.0/24"},
									},
								},
							},
						},
					},
				},
			},
		},
	}}

	got, err := extractASNMappings(obj)
	require.NoError(t, err)
	require.Equal(t, map[string]uint32{
		"10.128.0.0/14":   64512,
		"10.128.2.0/24":   64512,
		"192.168.99.0/24": 64512, // from toAdvertise, still local ASN
	}, got)
	// toReceive must not contribute peer ASN ownership
	_, hasRemote := got["203.0.113.0/24"]
	require.False(t, hasRemote)
}

func TestExtractASNMappings_SkipsASNZero(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"bgp": map[string]interface{}{
				"routers": []interface{}{
					map[string]interface{}{
						"asn":      int64(0),
						"prefixes": []interface{}{"10.0.0.0/8"},
					},
				},
			},
		},
	}}
	got, err := extractASNMappings(obj)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestExtractASNMappings_MultiVRF(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "multi", "namespace": "ns"},
		"spec": map[string]interface{}{
			"bgp": map[string]interface{}{
				"routers": []interface{}{
					map[string]interface{}{
						"asn":      int64(64512),
						"prefixes": []interface{}{"10.128.0.0/14"},
					},
					map[string]interface{}{
						"asn":      int64(64513),
						"vrf":      "blue",
						"prefixes": []interface{}{"10.200.0.0/16"},
					},
				},
			},
		},
	}}
	got, err := extractASNMappings(obj)
	require.NoError(t, err)
	require.Equal(t, map[string]uint32{
		"10.128.0.0/14": 64512,
		"10.200.0.0/16": 64513,
	}, got)
}
