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
	"os"
	"path"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/location"
	netdb "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/netdb"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

func getMockNetworkTransformRules() api.NetworkTransformRules {
	return api.NetworkTransformRules{
		api.NetworkTransformRule{
			Type: "add_subnet",
			AddSubnet: &api.NetworkAddSubnetRule{
				Input:      "srcIP",
				Output:     "subnet16SrcIP",
				SubnetMask: "/16",
			},
		},
		api.NetworkTransformRule{
			Type: "add_subnet",
			AddSubnet: &api.NetworkAddSubnetRule{
				Input:      "srcIP",
				Output:     "subnet24SrcIP",
				SubnetMask: "/24",
			},
		},
		api.NetworkTransformRule{
			Type: "add_subnet",
			AddSubnet: &api.NetworkAddSubnetRule{
				Input:      "emptyIP",
				Output:     "cidr_fail_skip",
				SubnetMask: "/16",
			},
		},
		api.NetworkTransformRule{
			Type: "add_service",
			AddService: &api.NetworkAddServiceRule{
				Input:    "dstPort",
				Output:   "service",
				Protocol: "protocol",
			},
		},
		api.NetworkTransformRule{
			Type: "add_service",
			AddService: &api.NetworkAddServiceRule{
				Input:    "dstPort",
				Output:   "service_protocol_num",
				Protocol: "protocol_num",
			},
		},
		api.NetworkTransformRule{
			Type: "add_service",
			AddService: &api.NetworkAddServiceRule{
				Input:    "srcPort",
				Output:   "unknown_service",
				Protocol: "protocol",
			},
		},
		api.NetworkTransformRule{
			Type: "add_location",
			AddLocation: &api.NetworkGenericRule{
				Input:  "8888IP",
				Output: "8888IP_location",
			},
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
		"8888IP_location_Longitude":       "-122.083847",
		"8888IP_location_Latitude":        "37.386051",
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
        - type: add_subnet
          add_subnet:
            input: srcIP
            output: subnetSrcIP
            subnet_mask: /24
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
	newTransform, err := NewTransformNetwork(config, nil)
	require.NoError(t, err)
	return newTransform
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
					{Type: api.NetworkAddSubnetLabel, AddSubnetLabel: &api.NetworkAddSubnetLabelRule{Input: "addr1", Output: "cat1"}},
					{Type: api.NetworkAddSubnetLabel, AddSubnetLabel: &api.NetworkAddSubnetLabelRule{Input: "addr2", Output: "cat2"}},
					{Type: api.NetworkAddSubnetLabel, AddSubnetLabel: &api.NetworkAddSubnetLabelRule{Input: "addr3", Output: "cat3"}},
					{Type: api.NetworkAddSubnetLabel, AddSubnetLabel: &api.NetworkAddSubnetLabelRule{Input: "addr4", Output: "cat4"}},
				},
				SubnetLabels: []api.NetworkTransformSubnetLabel{{
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

	tr, err := NewTransformNetwork(cfg, nil)
	require.NoError(t, err)

	output, ok := tr.Transform(entry)
	require.True(t, ok)
	require.Equal(t, config.GenericMap{
		"addr1": "10.1.2.3",
		"cat1":  "Pods overlay",
		"addr2": "100.1.2.3",
		"cat2":  "MySite.com",
		"addr3": "100.2.3.4",
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

	tr, err := NewTransformNetwork(cfg, nil)
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
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.3",
		"FlowDirection": "whatever",
	})
	require.True(t, ok)
	// Inner node => inner (2)
	require.Equal(t, config.GenericMap{
		"ReporterIP":    "10.1.2.3",
		"SrcHostIP":     "10.1.2.3",
		"DstHostIP":     "10.1.2.3",
		"IfDirection":   "whatever",
		"FlowDirection": 2,
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
	}, nil)
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
	}, nil)
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
	}, nil)
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
	}, nil)
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
	}, nil)
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
