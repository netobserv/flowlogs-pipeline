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
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/location"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
)

func getMockNetworkTransformRules() api.NetworkTransformRules {
	rules := api.NetworkTransformRules{
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
		api.NetworkTransformRule{
			Input:  "srcIP",
			Output: "srcIP_k8s",
			Type:   "add_kubernetes",
		},
	}

	return rules
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
		"value":                           "7",
		"smaller_than_10":                 "7",
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

func Test_Transform(t *testing.T) {
	entry := test.GetIngestMockEntry(false)
	rules := getMockNetworkTransformRules()
	expectedOutput := getExpectedOutput()

	var networkTransform = Network{
		api.TransformNetwork{
			Rules: rules,
		},
	}

	err := location.InitLocationDB()
	require.NoError(t, err)

	output := networkTransform.Transform(entry)

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
		api.TransformNetwork{
			Rules: rules,
		},
	}

	err := location.InitLocationDB()
	require.NoError(t, err)

	output := networkTransform.Transform(entry)

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
	v := test.InitConfig(t, string(yamlConfig))
	require.NotEqual(t, nil, v)

	newNetworkTransform := InitNewTransformNetwork(t, string(yamlConfig)).(*Network)
	require.NotEqual(t, nil, newNetworkTransform)

	entry := test.GetIngestMockEntry(false)
	output := newNetworkTransform.Transform(entry)

	require.Equal(t, "10.0.0.1", output["srcIP"])
	require.Equal(t, "10.0.0.0/24", output["subnetSrcIP"])
}

func InitNewTransformNetwork(t *testing.T, configFile string) Transformer {
	v := test.InitConfig(t, configFile)
	require.NotNil(t, v)

	config := config.Parameters[0].Transform
	newTransform, err := NewTransformNetwork(config)
	require.NoError(t, err)
	return newTransform
}

func Test_ConnTrackingTransformNetwork(t *testing.T) {
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
        - input: "{{.srcIP}},{{.srcPort}},{{.dstIP}},{{.dstPort}},{{.protocol}}"
          output: isNewFlow
          type: conn_tracking
          parameters: "777"
  - name: write1
    write:
      type: stdout
`)
	newNetworkTransform := InitNewTransformNetwork(t, string(yamlConfig)).(*Network)
	require.NotNil(t, newNetworkTransform)

	// first time flow is new
	entry := test.GetIngestMockEntry(false)
	output := newNetworkTransform.Transform(entry)
	require.Equal(t, "777", output["isNewFlow"])

	// second time, same flow is not new
	entry = test.GetIngestMockEntry(false)
	output = newNetworkTransform.Transform(entry)
	require.Equal(t, nil, output["isNewFlow"])
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
	v := test.InitConfig(t, string(yamlConfig))
	require.NotNil(t, v)

	newNetworkTransform := InitNewTransformNetwork(t, string(yamlConfig)).(*Network)
	require.NotNil(t, newNetworkTransform)

	entry := test.GetIngestMockEntry(false)
	output := newNetworkTransform.Transform(entry)

	require.Equal(t, "10.0.0.1", output["srcIP"])
	require.Equal(t, "10.0.0.0/24", output["subnetSrcIP"])
	require.Equal(t, "10.0.0.0/24", output["match-10.0.*"])
	require.NotEqual(t, "10.0.0.0/24", output["match-11.0.*"])
}
