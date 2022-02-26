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

package encode

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func initNewEncodeJson(t *testing.T) Encoder {
	newEncode, err := NewEncodeJson()
	require.NoError(t, err)
	return newEncode
}

func TestEncodeJson(t *testing.T) {
	newEncode := initNewEncodeJson(t)
	encodeByteArray := newEncode.(*encodeJson)
	map1 := config.GenericMap{
		"varInt":    12,
		"varString": "testString1",
		"varBool":   true,
	}
	map2 := config.GenericMap{
		"varString": "testString2",
		"varInt":    14,
		"varBool":   false,
	}
	map3 := config.GenericMap{}
	var out []interface{}
	var in []config.GenericMap
	out = encodeByteArray.Encode(in)
	require.Equal(t, 0, len(out))
	in = append(in, map1)
	in = append(in, map2)
	in = append(in, map3)
	out = encodeByteArray.Encode(in)
	require.Equal(t, len(in), len(out))
	expected1 := []byte(`{"varInt":12,"varBool":true,"varString":"testString1"}`)
	expected2 := []byte(`{"varInt":14,"varBool":false,"varString":"testString2"}`)
	expected3 := []byte(`{}`)
	require.JSONEq(t, string(expected1), string(out[0].([]byte)))
	require.JSONEq(t, string(expected2), string(out[1].([]byte)))
	require.JSONEq(t, string(expected3), string(out[2].([]byte)))
}
