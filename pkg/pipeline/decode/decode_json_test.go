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

	"github.com/stretchr/testify/require"
)

func initNewDecodeJSON(t *testing.T) Decoder {
	newDecode, err := NewDecodeJSON()
	require.Equal(t, nil, err)
	return newDecode
}

func TestDecodeJSON(t *testing.T) {
	newDecode := initNewDecodeJSON(t)
	decodeJSON := newDecode.(*DecodeJSON)

	out, err := decodeJSON.Decode([]byte(
		"{\"varInt\": 12, \"varString\":\"testString\", \"varBool\":false}"))
	require.NoError(t, err)
	require.Equal(t, float64(12), out["varInt"])
	require.Equal(t, "testString", out["varString"])
	require.Equal(t, false, out["varBool"])
	// TimeReceived is added if it does not exist
	require.NotZero(t, out["TimeReceived"])

	out, err = decodeJSON.Decode([]byte(
		"{\"varInt\": 14, \"varString\":\"testString2\", \"varBool\":true, \"TimeReceived\":12345}"))
	require.NoError(t, err)
	// TimeReceived is kept if it already existed
	require.EqualValues(t, 12345, out["TimeReceived"])

	// TODO: Check for more complicated json structures
}

func TestDecodeJSONTimestamps(t *testing.T) {
	newDecode := initNewDecodeJSON(t)
	decodeJSON := newDecode.(*DecodeJSON)
	out, err := decodeJSON.Decode([]byte("{\"unixTime\": 1645104030 }"))
	require.NoError(t, err)
	require.Equal(t, float64(1645104030), out["unixTime"])
}
