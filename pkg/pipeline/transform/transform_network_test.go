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

package transform

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/location"
	netdb "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/netdb"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getMockNetworkTransformRules() api.NetworkTransformRules {
	return api.NetworkTransformRules{
		api.NetworkTransformRule{
			Input:      "srcIP",
			Output:     "subnet16SrcIP",
			Type:       "add_subnet",
			Parameters: "/16",
		},
		api.NetworkTransformRule{
			Input:      "srcIP",
			Output:     "subnet24SrcIP",
			Type:       "add_subnet",
			Parameters: "/24",
		},
		api.NetworkTransformRule{
			Input:      "emptyIP",
			Output:     "cidr_fail_skip",
			Type:       "add_subnet",
			Parameters: "/16",
		},
		api.NetworkTransformRule{
			Input:      "value",
			Output:     "bigger_than_10",
			Type:       "add_if",
			Parameters: ">10",
		},
		api.NetworkTransformRule{
			Input:      "value",
			Output:     "smaller_than_10",
			Type:       "add_if",
			Parameters: "<10",
		},
		api.NetworkTransformRule{
			Input:      "dstPort",
			Output:     "service",
			Type:       "add_service",
			Parameters: "protocol",
		},
		api.NetworkTransformRule{
			Input:      "dstPort",
			Output:     "service_protocol_num",
			Type:       "add_service",
			Parameters: "protocol_num",
		},
		api.NetworkTransformRule{
			Input:      "srcPort",
			Output:     "unknown_service",
			Type:       "add_service",
			Parameters: "protocol",
		},
		api.NetworkTransformRule{
			Input:  "8888IP",
			Output: "8888IP_location",
			Type:   "add_location",
		},
	}
}

func getExpectedOutput() config.GenericMap {
	return config.GenericMap{
		"level":                           "error",
		"protocol":                        "tcp",
		"protocol_num":                    6,
		"message":                         "test message",
		"dstIP":                           "20.0.0.2",
		"dstPort":                         22,
		"srcIP":                           "10.0.0.1",
		"subnet16SrcIP":                   "10.0.0.0/16",
		"subnet24SrcIP":                   "10.0.0.0/24",
		"emptyIP":                         "",
		"value":                           7.0,
		"smaller_than_10":                 7.0,
		"smaller_than_10_Evaluate":        true,
		"service":                         "ssh",
		"service_protocol_num":            "ssh",
		"srcPort":                         11777,
		"8888IP":                          "8.8.8.8",
		"8888IP_location_CountryName":     "US",
		"8888IP_location_CountryLongName": "United States of America",
		"8888IP_location_RegionName":      "California",
		"8888IP_location_CityName":        "Mountain View",
		"8888IP_location_Longitude":       "-122.078514",
		"8888IP_location_Latitude":        "37.405991",
	}
}

func getServicesDB(t *testing.T) *netdb.ServiceNames {
	etcProtos, err := os.Open(path.Join("netdb", "testdata", "etcProtocols.txt"))
	require.NoError(t, err)
	defer etcProtos.Close()
	etcSvcs, err := os.Open(path.Join("netdb", "testdata", "etcServices.txt"))
	require.NoError(t, err)
	defer etcSvcs.Close()

	db, err := netdb.LoadServicesDB(etcProtos, etcSvcs)
	require.NoError(t, err)

	return db
}

func Test_Transform(t *testing.T) {
	entry := test.GetIngestMockEntry(false)
	rules := getMockNetworkTransformRules()
	expectedOutput := getExpectedOutput()

	var networkTransform = Network{
		TransformNetwork: api.TransformNetwork{
			Rules: rules,
		},
		svcNames: getServicesDB(t),
	}

	err := location.InitLocationDB()
	require.NoError(t, err)

	output, ok := networkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, expectedOutput, output)
}

func Test_TransformAddSubnetParseCIDRFailure(t *testing.T) {
	entry := test.GetIngestMockEntry(false)
	entry["srcIP"] = ""
	rules := getMockNetworkTransformRules()
	expectedOutput := getExpectedOutput()
	expectedOutput["srcIP"] = ""
	delete(expectedOutput, "subnet16SrcIP")
	delete(expectedOutput, "subnet24SrcIP")

	var networkTransform = Network{
		TransformNetwork: api.TransformNetwork{
			Rules: rules,
		},
		svcNames: getServicesDB(t),
	}

	err := location.InitLocationDB()
	require.NoError(t, err)

	output, ok := networkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, expectedOutput, output)
}

func Test_NewTransformNetwork(t *testing.T) {
	var yamlConfig = []byte(`
log-level: debug
pipeline:
  - name: transform1
  - name: write1
    follows: transform1
parameters:
  - name: transform1
    transform:
      type: network
      network:
        rules:
        - input: srcIP
          output: subnetSrcIP
          type: add_subnet
          parameters: /24
  - name: write1
    write:
      type: stdout
`)
	newNetworkTransform := InitNewTransformNetwork(t, string(yamlConfig)).(*Network)
	require.NotNil(t, newNetworkTransform)

	entry := test.GetIngestMockEntry(false)
	output, ok := newNetworkTransform.Transform(entry)
	require.True(t, ok)

	require.Equal(t, "10.0.0.1", output["srcIP"])
	require.Equal(t, "10.0.0.0/24", output["subnetSrcIP"])
}

func InitNewTransformNetwork(t *testing.T, configFile string) Transformer {
	v, cfg := test.InitConfig(t, configFile)
	require.NotNil(t, v)
	config := cfg.Parameters[0]
	newTransform, err := NewTransformNetwork(config)
	require.NoError(t, err)
	return newTransform
}

func Test_TransformNetworkDependentRulesAddRegExIf(t *testing.T) {
	var yamlConfig = []byte(`
log-level: debug
pipeline:
  - name: transform1
  - name: write1
    follows: transform1
parameters:
  - name: transform1
    transform:
      type: network
      network:
        rules:
        - input: srcIP
          output: subnetSrcIP
          type: add_subnet
          parameters: /24
        - input: subnetSrcIP
          output: match-10.0.*
          type: add_regex_if
          parameters: 10.0.*
        - input: subnetSrcIP
          output: match-11.0.*
          type: add_regex_if
          parameters: 11.0.*
  - name: write1
    write:
      type: stdout
`)
	newNetworkTransform := InitNewTransformNetwork(t, string(yamlConfig)).(*Network)
	require.NotNil(t, newNetworkTransform)

	entry := test.GetIngestMockEntry(false)
	output, ok := newNetworkTransform.Transform(entry)
	require.True(t, ok)

	require.Equal(t, "10.0.0.1", output["srcIP"])
	require.Equal(t, "10.0.0.0/24", output["subnetSrcIP"])
	require.Equal(t, "10.0.0.0/24", output["match-10.0.*"])
	require.NotEqual(t, "10.0.0.0/24", output["match-11.0.*"])
}

func Test_Transform_AddIfScientificNotation(t *testing.T) {
	newNetworkTransform := Network{
		TransformNetwork: api.TransformNetwork{
			Rules: api.NetworkTransformRules{
				api.NetworkTransformRule{
					Input:      "value",
					Output:     "bigger_than_10",
					Type:       "add_if",
					Parameters: ">10",
				},
				api.NetworkTransformRule{
					Input:      "value",
					Output:     "smaller_than_10",
					Type:       "add_if",
					Parameters: "<10",
				},
				api.NetworkTransformRule{
					Input:      "value",
					Output:     "dir",
					Assignee:   "in",
					Type:       "add_if",
					Parameters: "==1",
				},
				api.NetworkTransformRule{
					Input:      "value",
					Output:     "dir",
					Assignee:   "out",
					Type:       "add_if",
					Parameters: "==0",
				},
			},
		},
		svcNames: getServicesDB(t),
	}

	var entry config.GenericMap
	entry = config.GenericMap{
		"value": 1.2345e67,
	}
	output, ok := newNetworkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, true, output["bigger_than_10_Evaluate"])
	require.Equal(t, 1.2345e67, output["bigger_than_10"])

	entry = config.GenericMap{
		"value": 1.2345e-67,
	}
	output, ok = newNetworkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, true, output["smaller_than_10_Evaluate"])
	require.Equal(t, 1.2345e-67, output["smaller_than_10"])

	entry = config.GenericMap{
		"value": 1,
	}
	output, ok = newNetworkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, true, output["dir_Evaluate"])
	require.Equal(t, "in", output["dir"])

	entry = config.GenericMap{
		"value": 0,
	}
	output, ok = newNetworkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, true, output["dir_Evaluate"])
	require.Equal(t, "out", output["dir"])
}

func TestTransform_K8sEmptyNamespace(t *testing.T) {
	kubernetes.Data = &fakeKubeData{}
	nt := Network{
		TransformNetwork: api.TransformNetwork{
			Rules: api.NetworkTransformRules{{
				Type:   api.OpAddKubernetes,
				Input:  "SrcAddr",
				Output: "SrcK8s",
			}, {
				Type:   api.OpAddKubernetes,
				Input:  "DstAddr",
				Output: "DstK8s",
			}},
		},
	}
	// We need to check that, whether it returns NotFound or just an empty namespace,
	// there is no map entry for that namespace (an empty-valued map entry is not valid)
	out, _ := nt.Transform(config.GenericMap{
		"SrcAddr": "1.2.3.4", // would return an empty namespace
		"DstAddr": "3.2.1.0", // would return NotFound
	})
	assert.NotContains(t, out, "SrcK8s_Namespace")
	assert.NotContains(t, out, "DstK8s_Namespace")
}

type fakeKubeData struct{}

func (d *fakeKubeData) InitFromConfig(_ string) error {
	return nil
}
func (*fakeKubeData) GetInfo(n string) (*kubernetes.Info, error) {
	// If found, returns an empty info (empty namespace)
	if n == "1.2.3.4" {
		return &kubernetes.Info{}, nil
	}
	return nil, errors.New("notFound")
}

func Test_Transform_Multiplier(t *testing.T) {
	newNetworkTransform := Network{
		TransformNetwork: api.TransformNetwork{
			Rules: api.NetworkTransformRules{
				api.NetworkTransformRule{
					Input:      "input_var",
					Output:     "output_var",
					Type:       "multiplier",
					Parameters: "10",
				},
			},
		},
	}

	var entry config.GenericMap
	entry = config.GenericMap{
		"input_var": 3,
	}
	output, ok := newNetworkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, 30, output["output_var"])

	entry = config.GenericMap{
		"input_var": 4.0,
	}
	output, ok = newNetworkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, 40.0, output["output_var"])

	entry = config.GenericMap{
		"input_var": "not_a_number",
	}
	_, ok = newNetworkTransform.Transform(entry)
	require.False(t, ok)

	entry = config.GenericMap{
		"input_var": true,
	}
	_, ok = newNetworkTransform.Transform(entry)
	require.False(t, ok)

	entry = config.GenericMap{
		"input_var": -4.0,
	}
	output, ok = newNetworkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, -40.0, output["output_var"])

	entry = config.GenericMap{
		"input_var": uint16(5),
	}
	output, ok = newNetworkTransform.Transform(entry)
	require.True(t, ok)
	require.Equal(t, uint16(50), output["output_var"])

	var badConfig = []byte(`
parameters:
  - name: transform1
    transform:
      type: network
      network:
        rules:
        - input: bytes
          output: bytes
          type: multiplier
          parameters: "not_a_number"
`)
	v, cfg := test.InitConfig(t, string(badConfig))
	require.NotNil(t, v)

	config := cfg.Parameters[0]
	_, err := NewTransformNetwork(config)
	require.Error(t, err)
}
