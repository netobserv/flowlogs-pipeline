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

func Test_Categorize(t *testing.T) {
	entry := config.GenericMap{
		"addr1": "10.1.2.3",
		"addr2": "100.1.2.3",
		"addr3": "100.2.3.4",
		"addr4": "101.1.0.0",
	}
	cfg := config.StageParam{
		Transform: &config.Transform{
			Network: &api.TransformNetwork{
				Rules: []api.NetworkTransformRule{
					{Type: api.OpAddIPCategory, Input: "addr1", Output: "cat1"},
					{Type: api.OpAddIPCategory, Input: "addr2", Output: "cat2"},
					{Type: api.OpAddIPCategory, Input: "addr3", Output: "cat3"},
					{Type: api.OpAddIPCategory, Input: "addr4", Output: "cat4"},
				},
				IPCategories: []api.NetworkTransformIPCategory{{
					Name:  "Pods overlay",
					CIDRs: []string{"10.0.0.0/8"},
				}, {
					Name:  "MySite.com",
					CIDRs: []string{"101.1.0.0/32", "100.1.0.0/16"},
				}, {
					Name:  "MyOtherSite.com",
					CIDRs: []string{"100.2.3.10/32"},
				}},
			},
		},
	}

	tr, err := NewTransformNetwork(cfg)
	require.NoError(t, err)

	output, ok := tr.Transform(entry)
	require.True(t, ok)
	require.Equal(t, config.GenericMap{
		"addr1": "10.1.2.3",
		"cat1":  "Pods overlay",
		"addr2": "100.1.2.3",
		"cat2":  "MySite.com",
		"addr3": "100.2.3.4",
		"cat3":  "",
		"addr4": "101.1.0.0",
		"cat4":  "MySite.com",
	}, output)
}

func Test_ReinterpretDirection(t *testing.T) {
	cfg := config.StageParam{
		Transform: &config.Transform{
			Network: &api.TransformNetwork{
				Rules: []api.NetworkTransformRule{{
					Type: "reinterpret_direction",
				}},
				DirectionInfo: api.NetworkTransformDirectionInfo{
					ReporterIPField:    "ReporterIP",
					SrcHostField:       "SrcHostIP",
					DstHostField:       "DstHostIP",
					FlowDirectionField: "FlowDirection",
					IfDirectionField:   "IfDirection",
				},
			},
		},
	}

	tr, err := NewTransformNetwork(cfg)
	require.NoError(t, err)

	output, ok := tr.Transform(config.GenericMap{
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Source reporter => egress (1)
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"IfDirection":   "whatever",
		"FlowDirection": 1,
	}, output)

	output, ok = tr.Transform(config.GenericMap{
		"ReporterIP":    "10.1.2.4",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Destination reporter => ingress (0)
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.4",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"IfDirection":   "whatever",
		"FlowDirection": 0,
	}, output)

	output, ok = tr.Transform(config.GenericMap{
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Source reporter => egress (1)
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"IfDirection":   "whatever",
		"FlowDirection": 1,
	}, output)

	output, ok = tr.Transform(config.GenericMap{
		"ReporterIP":    "10.1.2.4",
		"DstHostIP":     "10.1.2.4",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Destination reporter => ingress (0)
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.4",
		"DstHostIP":     "10.1.2.4",
		"IfDirection":   "whatever",
		"FlowDirection": 0,
	}, output)

	output, ok = tr.Transform(config.GenericMap{
		"ReporterIP":    "10.1.2.100",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Missing or unknown reporter => FlowDirection same as IfDirection
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.100",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"IfDirection":   "whatever",
		"FlowDirection": "whatever",
	}, output)

	output, ok = tr.Transform(config.GenericMap{
		"ReporterIP":    "10.1.2.4",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Missing src and dst host ips => FlowDirection same as IfDirection
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.4",
		"IfDirection":   "whatever",
		"FlowDirection": "whatever",
	}, output)

	output, ok = tr.Transform(config.GenericMap{
		"SrcHostIP":     "10.1.2.3",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Missing Reporter and Dest => no change in FlowDirection (even though technically reporter == dest)
	require.Equal(t, config.GenericMap{
		"SrcHostIP":     "10.1.2.3",
		"IfDirection":   "whatever",
		"FlowDirection": "whatever",
	}, output)

	output, ok = tr.Transform(config.GenericMap{
		"ReporterIP": "10.1.2.3",
		"SrcHostIP":  "10.1.2.3",
		"DstHostIP":  "10.1.2.4",
	})
	require.True(t, ok)
	// Missing FlowDirection => no IfDirection either
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"FlowDirection": 1,
	}, output)
}

func Test_ValidateReinterpretDirection(t *testing.T) {
	// Missing reporter field
	_, err := NewTransformNetwork(config.StageParam{
		Transform: &config.Transform{
			Network: &api.TransformNetwork{
				Rules: []api.NetworkTransformRule{{
					Type: "reinterpret_direction",
				}},
				DirectionInfo: api.NetworkTransformDirectionInfo{
					SrcHostField:       "SrcHostIP",
					DstHostField:       "DstHostIP",
					FlowDirectionField: "FlowDirection",
					IfDirectionField:   "IfDirection",
				},
			},
		},
	})
	require.Contains(t, err.Error(), "missing ReporterIPField")

	// Missing src field
	_, err = NewTransformNetwork(config.StageParam{
		Transform: &config.Transform{
			Network: &api.TransformNetwork{
				Rules: []api.NetworkTransformRule{{
					Type: "reinterpret_direction",
				}},
				DirectionInfo: api.NetworkTransformDirectionInfo{
					ReporterIPField:    "ReporterIP",
					DstHostField:       "DstHostIP",
					FlowDirectionField: "FlowDirection",
					IfDirectionField:   "IfDirection",
				},
			},
		},
	})
	require.Contains(t, err.Error(), "missing SrcHostField")

	// Missing dst field
	_, err = NewTransformNetwork(config.StageParam{
		Transform: &config.Transform{
			Network: &api.TransformNetwork{
				Rules: []api.NetworkTransformRule{{
					Type: "reinterpret_direction",
				}},
				DirectionInfo: api.NetworkTransformDirectionInfo{
					ReporterIPField:    "ReporterIP",
					SrcHostField:       "SrcHostIP",
					FlowDirectionField: "FlowDirection",
					IfDirectionField:   "IfDirection",
				},
			},
		},
	})
	require.Contains(t, err.Error(), "missing DstHostField")

	// Missing flow direction field
	_, err = NewTransformNetwork(config.StageParam{
		Transform: &config.Transform{
			Network: &api.TransformNetwork{
				Rules: []api.NetworkTransformRule{{
					Type: "reinterpret_direction",
				}},
				DirectionInfo: api.NetworkTransformDirectionInfo{
					ReporterIPField:  "ReporterIP",
					SrcHostField:     "SrcHostIP",
					DstHostField:     "DstHostIP",
					IfDirectionField: "IfDirection",
				},
			},
		},
	})
	require.Contains(t, err.Error(), "missing FlowDirectionField")

	// Missing if direction field does not trigger an error (this field will just not be populated)
	tr, err := NewTransformNetwork(config.StageParam{
		Transform: &config.Transform{
			Network: &api.TransformNetwork{
				Rules: []api.NetworkTransformRule{{
					Type: "reinterpret_direction",
				}},
				DirectionInfo: api.NetworkTransformDirectionInfo{
					ReporterIPField:    "ReporterIP",
					SrcHostField:       "SrcHostIP",
					DstHostField:       "DstHostIP",
					FlowDirectionField: "FlowDirection",
				},
			},
		},
	})
	require.NoError(t, err)

	// Test transformation when IfDirection is missing
	output, ok := tr.Transform(config.GenericMap{
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Source reporter => egress (1)
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.4",
		"FlowDirection": 1,
	}, output)
}
