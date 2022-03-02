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

func initNewEncodeNone(t *testing.T) Encoder {
	newEncode, err := NewEncodeNone()
	require.Equal(t, err, nil)
	return newEncode
}

func TestEncodeNone(t *testing.T) {
	newEncode := initNewEncodeNone(t)
	encodeNone := newEncode.(*encodeNone)
	map1 := config.GenericMap{
		"varInt":    12,
		"varString": "string1",
		"varbool":   true,
	}
	map2 := config.GenericMap{
		"varInt":    14,
		"varString": "string2",
		"varbool":   false,
	}
	map3 := config.GenericMap{}
	var out []config.GenericMap
	var in []config.GenericMap
	out = encodeNone.Encode(in)
	require.Equal(t, 0, len(out))
	in = append(in, map1)
	in = append(in, map2)
	in = append(in, map3)
	out = encodeNone.Encode(in)
	require.Equal(t, len(in), len(out))
}
