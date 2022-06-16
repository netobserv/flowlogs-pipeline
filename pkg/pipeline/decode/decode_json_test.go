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
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
)

func initNewDecodeJson(t *testing.T) Decoder {
	newDecode, err := NewDecodeJson()
	require.Equal(t, nil, err)
	return newDecode
}

func TestDecodeJson(t *testing.T) {
	newDecode := initNewDecodeJson(t)
	decodeJson := newDecode.(*DecodeJson)
	inputString1 := "{\"varInt\": 12, \"varString\":\"testString\", \"varBool\":false}"
	inputString2 := "{\"varInt\": 14, \"varString\":\"testString2\", \"varBool\":true, \"TimeReceived\":12345}"
	inputString3 := "{}"
	inputStringErr := "{\"varInt\": 14, \"varString\",\"testString2\", \"varBool\":true}"
	var in []interface{}
	var out []config.GenericMap
	out = decodeJson.Decode(in)
	require.Equal(t, 0, len(out))
	in = append(in, inputString1)
	in = append(in, inputString2)
	in = append(in, inputString3)
	in = append(in, inputStringErr)
	out = decodeJson.Decode(in)
	require.Equal(t, len(out), 3)
	require.Equal(t, float64(12), out[0]["varInt"])
	require.Equal(t, "testString", out[0]["varString"])
	require.Equal(t, false, out[0]["varBool"])
	// TimeReceived is added if it does not exist
	require.NotZero(t, out[0]["TimeReceived"])
	// TimeReceived is kept if it already existed
	require.EqualValues(t, 12345, out[1]["TimeReceived"])

	// TODO: Check for more complicated json structures
}

func TestDecodeJsonTimestamps(t *testing.T) {
	newDecode := initNewDecodeJson(t)
	decodeJson := newDecode.(*DecodeJson)
	inputString1 := "{\"unixTime\": 1645104030 }"
	var in []interface{}
	var out []config.GenericMap
	out = decodeJson.Decode(in)
	require.Equal(t, 0, len(out))
	in = append(in, inputString1)
	out = decodeJson.Decode(in)
	require.Equal(t, len(out), 1)
	require.Equal(t, float64(1645104030), out[0]["unixTime"])
}
