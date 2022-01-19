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
	"github.com/stretchr/testify/require"
	"testing"
)

func initNewDecodeJson(t *testing.T) Decoder {
	newDecode, err := NewDecodeJson()
	require.Equal(t, err, nil)
	return newDecode
}

func TestDecodeJson(t *testing.T) {
	newDecode := initNewDecodeJson(t)
	decodeJson := newDecode.(*decodeJson)
	inputString1 := "{\"varInt\": 12, \"varString\":\"testString\", \"varBool\":false}"
	inputString2 := "{\"varInt\": 12, \"varString\":\"testString\", \"varBool\":false}"
	var in []interface{}
	in = append(in, inputString1)
	in = append(in, inputString2)
	out := decodeJson.Decode(in)
	require.Equal(t, len(out), 2)
	// verify that all items come back as strings
	require.Equal(t, out[0]["varInt"], "12")
	require.Equal(t, out[0]["varString"], "testString")
	require.Equal(t, out[0]["varBool"], "false")

	// TODO: Check for more complicated json structures
	// TODO: check for error cases
}
