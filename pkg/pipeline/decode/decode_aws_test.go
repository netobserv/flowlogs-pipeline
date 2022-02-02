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

package decode

import (
	"bufio"
	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

const testConfig1 = `
pipeline:
  decode:
    type: aws
`
const testConfig2 = `
pipeline:
  decode:
    type: aws
    aws:
      fields:
        - version
        - vpc-id
        - subnet-id
        - instance-id
        - interface-id
        - account-id
        - type
        - srcaddr
        - dstaddr
        - srcport
        - dstport
        - pkt-srcaddr
        - pkt-dstaddr
        - protocol
        - bytes
        - packets
        - start
        - end
        - action
        - tcp-flags
        - log-status
`

const testConfigErr = `
pipeline:
  decode:
    type: aws
    aws:
      fields:
        version
        vpc-id
`

// aws version 2 standard format
const input1 = `2 123456789010 eni-1235b8ca123456789 172.31.16.139 172.31.16.21 20641 22 6 20 4249 1418530010 1418530070 ACCEPT OK
2 123456789010 eni-1235b8ca123456789 172.31.9.69 172.31.9.12 49761 3389 6 20 4249 1418530010 1418530070 REJECT OK
2 123456789010 eni-1235b8ca123456789 203.0.113.12 172.31.16.139 0 0 1 4 336 1432917027 1432917142 ACCEPT OK
`

const inputErr = `2 123456789010 eni-1235b8ca123456789 172.31.16.139 172.31.16.21 20641 22 6 20 4249 1418530010 1418530070 ACCEPT OK
2 12345 6789010 eni-1235b8ca123456789 172.31.9.69 172.31.9.12 49761 3389 6 20 4249 1418530010 1418530070 REJECT OK
2 123456789010 eni-1235b8ca123456789 203.0.113.12 172.31.16.139 0 0 1 4 336 1432917027 1432917142 ACCEPT OK
`

// aws version 3 custom format
const input2 = `3 vpc-abcdefab012345678 subnet-aaaaaaaa012345678 i-01234567890123456 eni-1235b8ca123456789 123456789010 IPv4 52.213.180.42 10.0.0.62 43416 5001 52.213.180.42 10.0.0.62 6 568 8 1566848875 1566848933 ACCEPT 2 OK
3 vpc-abcdefab012345678 subnet-aaaaaaaa012345678 i-01234567890123456 eni-1235b8ca123456789 123456789010 IPv4 10.0.0.62 52.213.180.42 5001 43416 10.0.0.62 52.213.180.42 6 376 7 1566848875 1566848933 ACCEPT 18 OK
`

func initNewDecodeAws(t *testing.T, testConfig string) Decoder {
	v := test.InitConfig(t, testConfig)
	val := v.Get("pipeline.decode.type")
	require.Equal(t, "aws", val)

	tmp := v.Get("pipeline.decode.aws")
	if tmp != nil {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		b, err := json.Marshal(&tmp)
		require.Equal(t, err, nil)
		// perform initializations usually done in main.go
		config.Opt.PipeLine.Decode.Aws = string(b)
	} else {
		config.Opt.PipeLine.Decode.Aws = ""
	}

	newDecode, err := NewDecodeAws()
	require.Equal(t, nil, err)
	return newDecode
}

func breakIntoLines(input string) []interface{} {
	lines := make([]interface{}, 0)
	r := strings.NewReader(input)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		lines = append(lines, text)
	}
	return lines
}

func TestDecodeAwsDefault(t *testing.T) {
	newDecode := initNewDecodeAws(t, testConfig1)
	decodeAws := newDecode.(*decodeAws)
	require.Equal(t, defaultKeys, decodeAws.keyTags)

	lines := breakIntoLines(input1)
	nLines := len(lines)
	output := decodeAws.Decode(lines)
	require.Equal(t, nLines, len(output))

	expectedResult := config.GenericMap{
		"version":      "2",
		"account-id":   "123456789010",
		"interface-id": "eni-1235b8ca123456789",
		"srcaddr":      "172.31.16.139",
		"dstaddr":      "172.31.16.21",
		"srcport":      "20641",
		"dstport":      "22",
		"protocol":     "6",
		"packets":      "20",
		"bytes":        "4249",
		"start":        "1418530010",
		"end":          "1418530070",
		"action":       "ACCEPT",
		"log-status":   "OK",
	}
	require.Equal(t, expectedResult, output[0])
}

func TestDecodeAwsDefaultErr(t *testing.T) {
	newDecode := initNewDecodeAws(t, testConfig1)
	decodeAws := newDecode.(*decodeAws)
	require.Equal(t, defaultKeys, decodeAws.keyTags)

	lines := breakIntoLines(inputErr)
	nLines := len(lines) - 1
	output := decodeAws.Decode(lines)
	require.Equal(t, nLines, len(output))
}

func TestDecodeAwsCustom(t *testing.T) {
	newDecode := initNewDecodeAws(t, testConfig2)
	decodeAws := newDecode.(*decodeAws)

	lines := breakIntoLines(input2)
	nLines := len(lines)
	output := decodeAws.Decode(lines)
	require.Equal(t, nLines, len(output))

	expectedResult := config.GenericMap{
		"version":      "3",
		"vpc-id":       "vpc-abcdefab012345678",
		"subnet-id":    "subnet-aaaaaaaa012345678",
		"instance-id":  "i-01234567890123456",
		"interface-id": "eni-1235b8ca123456789",
		"account-id":   "123456789010",
		"type":         "IPv4",
		"srcaddr":      "52.213.180.42",
		"dstaddr":      "10.0.0.62",
		"srcport":      "43416",
		"dstport":      "5001",
		"pkt-srcaddr":  "52.213.180.42",
		"pkt-dstaddr":  "10.0.0.62",
		"protocol":     "6",
		"bytes":        "568",
		"packets":      "8",
		"start":        "1566848875",
		"end":          "1566848933",
		"action":       "ACCEPT",
		"tcp-flags":    "2",
		"log-status":   "OK",
	}
	require.Equal(t, expectedResult, output[0])
}

func TestConfigErr(t *testing.T) {
	v := test.InitConfig(t, testConfigErr)
	val := v.Get("pipeline.decode.type")
	require.Equal(t, "aws", val)

	tmp := v.Get("pipeline.decode.aws")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(&tmp)
	require.Equal(t, err, nil)
	// perform initializations usually done in main.go
	config.Opt.PipeLine.Decode.Aws = string(b)

	newDecode, _ := NewDecodeAws()
	require.Equal(t, nil, newDecode)
}
