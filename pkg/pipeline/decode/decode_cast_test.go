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

func initNewDecodeCast(t *testing.T) Decoder {
	newDecode, err := NewDecodeCast()
	require.Equal(t, nil, err)
	return newDecode
}

func TestDecodeCast(t *testing.T) {
	newDecode := initNewDecodeCast(t)
	decodeCast := newDecode.(*DecodeCast)
	inputMap1 := map[string]interface{}{
		"varInt":    12,
		"varString": "testString",
		"varBool":   false,
	}
	inputMap2 := map[string]interface{}{
		"varInt":    14,
		"varString": "testString2",
		"varBool":   true,
	}
	inputMap3 := map[string]interface{}{}
	var in []interface{}
	var out []config.GenericMap
	out = decodeCast.Decode(in)
	require.Equal(t, 0, len(out))
	in = append(in, inputMap1)
	in = append(in, inputMap2)
	in = append(in, inputMap3)
	out = decodeCast.Decode(in)
	require.Equal(t, len(out), 3)
	require.Equal(t, 12, out[0]["varInt"])
	require.Equal(t, "testString", out[0]["varString"])
	require.Equal(t, false, out[0]["varBool"])
}
