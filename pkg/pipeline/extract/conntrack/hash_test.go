/*
 * Copyright (C) 2022 IBM, Inc.
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
	"hash/fnv"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
)

var testHasher = fnv.New64a()

func TestComputeHash_Unidirectional(t *testing.T) {
	keyDefinition := api.KeyDefinition{
		FieldGroups: []api.FieldGroup{
			{
				Name: "src",
				Fields: []string{
					"SrcAddr",
					"SrcPort",
				},
			},
			{
				Name: "dst",
				Fields: []string{
					"DstAddr",
					"DstPort",
				},
			},
			{
				Name: "protocol",
				Fields: []string{
					"Proto",
				},
			},
		},
		Hash: api.ConnTrackHash{
			FieldGroupRefs: []string{"src", "dst", "protocol"},
		},
	}
	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 9001
	portB := 9002
	protocolA := 6
	protocolB := 7
	flowDir := 0
	table := []struct {
		name     string
		flowLog1 config.GenericMap
		flowLog2 config.GenericMap
		sameHash bool
	}{
		{
			"Same IP, port and protocol",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 222, 11, false),
			true,
		},
		{
			"Alternating ip+port",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipB, portB, ipA, portA, protocolA, flowDir, 222, 11, false),
			false,
		},
		{
			"Alternating ip",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipB, portA, ipA, portB, protocolA, flowDir, 222, 11, false),
			false,
		},
		{
			"Alternating port",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipA, portB, ipB, portA, protocolA, flowDir, 222, 11, false),
			false,
		},
		{
			"Same IP+port, different protocol",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipA, portA, ipB, portB, protocolB, flowDir, 222, 11, false),
			false,
		},
	}
	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			h1, err1 := ComputeHash(test.flowLog1, keyDefinition, testHasher)
			h2, err2 := ComputeHash(test.flowLog2, keyDefinition, testHasher)
			require.NoError(t, err1)
			require.NoError(t, err2)
			if test.sameHash {
				require.Equal(t, h1, h2)
			} else {
				require.NotEqual(t, h1, h2)
			}
		})
	}
}

func TestComputeHash_Bidirectional(t *testing.T) {
	keyDefinition := api.KeyDefinition{
		FieldGroups: []api.FieldGroup{
			{
				Name: "src",
				Fields: []string{
					"SrcAddr",
					"SrcPort",
				},
			},
			{
				Name: "dst",
				Fields: []string{
					"DstAddr",
					"DstPort",
				},
			},
			{
				Name: "protocol",
				Fields: []string{
					"Proto",
				},
			},
		},
		Hash: api.ConnTrackHash{
			FieldGroupRefs: []string{"protocol"},
			FieldGroupARef: "src",
			FieldGroupBRef: "dst",
		},
	}
	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 1
	portB := 9002
	protocolA := 6
	protocolB := 7
	flowDir := 0
	table := []struct {
		name     string
		flowLog1 config.GenericMap
		flowLog2 config.GenericMap
		sameHash bool
	}{
		{
			"Same IP, port and protocol",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 222, 11, false),
			true,
		},
		{
			"Alternating ip+port",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipB, portB, ipA, portA, protocolA, flowDir, 222, 11, false),
			true,
		},
		{
			"Alternating ip",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipB, portA, ipA, portB, protocolA, flowDir, 222, 11, false),
			false,
		},
		{
			"Alternating port",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipA, portB, ipB, portA, protocolA, flowDir, 222, 11, false),
			false,
		},
		{
			"Same IP+port, different protocol",
			newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false),
			newMockFlowLog(ipA, portA, ipB, portB, protocolB, flowDir, 222, 11, false),
			false,
		},
	}
	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			h1, err1 := ComputeHash(test.flowLog1, keyDefinition, testHasher)
			h2, err2 := ComputeHash(test.flowLog2, keyDefinition, testHasher)
			require.NoError(t, err1)
			require.NoError(t, err2)
			if test.sameHash {
				require.Equal(t, h1.hashTotal, h2.hashTotal)
			} else {
				require.NotEqual(t, h1.hashTotal, h2.hashTotal)
			}
		})
	}
}

func TestComputeHash_MissingField(t *testing.T) {
	keyDefinition := api.KeyDefinition{
		FieldGroups: []api.FieldGroup{
			{
				Name: "src",
				Fields: []string{
					"SrcAddr",
					"Missing",
				},
			},
		},
		Hash: api.ConnTrackHash{
			FieldGroupRefs: []string{"src"},
		},
	}

	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	portA := 1
	portB := 9002
	protocolA := 6
	flowDir := 0

	fl := newMockFlowLog(ipA, portA, ipB, portB, protocolA, flowDir, 111, 22, false)

	h, err := ComputeHash(fl, keyDefinition, testHasher)
	require.NoError(t, err)
	require.NotNil(t, h)
}
