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
	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/api"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/transform/location"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
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
		"protocol_num":                    "6",
		"message":                         "test message",
		"dstIP":                           "20.0.0.2",
		"dstPort":                         "22",
		"srcIP":                           "10.0.0.1",
		"subnet16SrcIP":                   "10.0.0.0/16",
		"subnet24SrcIP":                   "10.0.0.0/24",
		"emptyIP":                         "",
		"value":                           "7",
		"smaller_than_10":                 "7",
		"smaller_than_10_Evaluate":        true,
		"service":                         "ssh",
		"service_protocol_num":            "ssh",
		"srcPort":                         "11777",
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
	_ = location.InitLocationDB()
	output := networkTransform.Transform(entry)

	require.Equal(t, output, expectedOutput)
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
	_ = location.InitLocationDB()
	output := networkTransform.Transform(entry)

	require.Equal(t, output, expectedOutput)
}

func Test_NewTransformNetwork(t *testing.T) {
	var yamlConfig = []byte(`
log-level: debug
pipeline:
  transform:
    - type: network
      network:
        rules:
        - input: srcIP
          output: subnetSrcIP
          type: add_subnet
          parameters: /24
  write:
    type: stdout
`)
	v := test.InitConfig(t, string(yamlConfig))
	val := v.Get("pipeline.transform")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.Equal(t, err, nil)
	config.Opt.PipeLine.Transform = string(b)

	newNetworkTransform := InitNewTransform(t, string(yamlConfig)).(*Network)
	require.Equal(t, err, nil)

	entry := test.GetIngestMockEntry(false)
	output := newNetworkTransform.Transform(entry)

	require.Equal(t, output["srcIP"], "10.0.0.1")
	require.Equal(t, output["subnetSrcIP"], "10.0.0.0/24")
}

func Test_TransformNetworkDependentRulesAddRegExIf(t *testing.T) {
	var yamlConfig = []byte(`
log-level: debug
pipeline:
  transform:
    - type: network
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
  write:
    type: stdout
`)
	v := test.InitConfig(t, string(yamlConfig))
	val := v.Get("pipeline.transform")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&val)
	require.Equal(t, err, nil)
	config.Opt.PipeLine.Transform = string(b)

	newNetworkTransform := InitNewTransform(t, string(yamlConfig)).(*Network)
	require.Equal(t, err, nil)

	entry := test.GetIngestMockEntry(false)
	output := newNetworkTransform.Transform(entry)

	require.Equal(t, output["srcIP"], "10.0.0.1")
	require.Equal(t, output["subnetSrcIP"], "10.0.0.0/24")
	require.Equal(t, output["match-10.0.*"], "10.0.0.0/24")
	require.NotEqual(t, output["match-11.0.*"], "10.0.0.0/24")
}
