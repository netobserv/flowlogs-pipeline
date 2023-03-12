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

package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func GetIngestMockEntry(missingKey bool) config.GenericMap {
	entry := config.GenericMap{
		"srcIP":        "10.0.0.1",
		"8888IP":       "8.8.8.8",
		"emptyIP":      "",
		"level":        "error",
		"srcPort":      11777,
		"protocol":     "tcp",
		"protocol_num": 6,
		"value":        7.0,
		"message":      "test message",
	}

	if !missingKey {
		entry["dstIP"] = "20.0.0.2"
		entry["dstPort"] = 22
	}

	return entry
}

func InitConfig(t *testing.T, conf string) (*viper.Viper, *config.ConfigFileStruct) {
	ResetPromRegistry()
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	yamlConfig := []byte(conf)
	v := viper.New()
	v.SetConfigType("yaml")
	r := bytes.NewReader(yamlConfig)
	err := v.ReadConfig(r)
	require.NoError(t, err)

	var b []byte
	pipelineStr := v.Get("pipeline")
	b, err = json.Marshal(&pipelineStr)
	if err != nil {
		fmt.Printf("error marshaling: %v\n", err)
		return nil, nil
	}
	opts := config.Options{}
	opts.PipeLine = string(b)
	parametersStr := v.Get("parameters")
	b, err = json.Marshal(&parametersStr)
	if err != nil {
		fmt.Printf("error marshaling: %v\n", err)
		return nil, nil
	}
	opts.Parameters = string(b)
	out, err := config.ParseConfig(opts)
	if err != nil {
		fmt.Printf("error in parsing config file: %v \n", err)
		return nil, nil
	}

	return v, &out
}

func GetExtractMockEntry() config.GenericMap {
	entry := config.GenericMap{
		"srcAddr": "10.1.2.3",
		"dstAddr": "10.1.2.4",
		"srcPort": 9001,
		"dstPort": 39504,
		"bytes":   1234,
		"packets": 34,
	}
	return entry
}

func CreateMockAgg(name, operationKey, groupByKeys, agg, op string, totalValue float64, totalCount int, rrv []float64, recentOpValue float64, recentCount int) config.GenericMap {
	return config.GenericMap{
		"name":              name,
		"operation_key":     operationKey,
		"by":                groupByKeys,
		"aggregate":         agg,
		groupByKeys:         agg,
		"operation_type":    api.AggregateOperation(op),
		"total_value":       totalValue,
		"recent_raw_values": rrv,
		"total_count":       totalCount,
		"recent_op_value":   recentOpValue,
		"recent_count":      recentCount,
	}
}

func RunCommand(command string) string {
	var cmd *exec.Cmd
	var outBuf bytes.Buffer
	var err error
	cmdStrings := strings.Split(command, " ")
	cmdBase := cmdStrings[0]
	cmdStrings = cmdStrings[1:]
	cmd = exec.Command(cmdBase, cmdStrings...)
	cmd.Stdout = &outBuf
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("error in running command: %v \n", err)
	}
	output := outBuf.Bytes()
	//strip newline from end of output
	if len(output) > 0 && output[len(output)-1] == '\n' {
		output = output[0 : len(output)-1]
	}
	fmt.Printf("output = %s\n", string(output))
	return string(output)
}

func DeserializeJSONToMap(t *testing.T, in string) config.GenericMap {
	t.Helper()
	var m config.GenericMap
	err := json.Unmarshal([]byte(in), &m)
	require.NoError(t, err)
	return m
}

func DumpToTemp(content string) (string, error, func()) {
	file, err := os.CreateTemp("", "flp-tests-")
	if err != nil {
		return "", err, nil
	}
	err = os.WriteFile(file.Name(), []byte(content), 0644)
	if err != nil {
		defer os.Remove(file.Name())
		return "", err, nil
	}
	return file.Name(), nil, func() {
		os.Remove(file.Name())
	}
}

func WaitFromChannel(in chan config.GenericMap, timeout time.Duration) (config.GenericMap, error) {
	timeoutReached := time.NewTicker(timeout)
	select {
	case record := <-in:
		return record, nil
	case <-timeoutReached.C:
		return nil, errors.New("Timeout reached")
	}
}

func GetExtractMockEntries2() []config.GenericMap {
	entries := []config.GenericMap{
		{"SrcAddr": "10.0.0.1", "DstAddr": "11.0.0.1", "Bytes": 100, "Packets": 1},
		{"SrcAddr": "10.0.0.2", "DstAddr": "11.0.0.1", "Bytes": 200, "Packets": 2},
		{"SrcAddr": "10.0.0.3", "DstAddr": "11.0.0.1", "Bytes": 300, "Packets": 3},
		{"SrcAddr": "10.0.0.1", "DstAddr": "11.0.0.1", "Bytes": 400, "Packets": 1},
		{"SrcAddr": "10.0.0.2", "DstAddr": "11.0.0.1", "Bytes": 500, "Packets": 1},
		{"SrcAddr": "10.0.0.3", "DstAddr": "11.0.0.1", "Bytes": 600, "Packets": 1},
		{"SrcAddr": "10.0.0.1", "DstAddr": "11.0.0.1", "Bytes": 700, "Packets": 4},
		{"SrcAddr": "10.0.0.2", "DstAddr": "11.0.0.1", "Bytes": 800, "Packets": 5},
		{"SrcAddr": "10.0.0.3", "DstAddr": "11.0.0.1", "Bytes": 900, "Packets": 1},
		{"SrcAddr": "10.0.0.4", "DstAddr": "11.0.0.1", "Bytes": 1000, "Packets": 1},
	}
	return entries
}

func ResetPromRegistry() {
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
}
