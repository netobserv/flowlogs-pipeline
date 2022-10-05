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
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
)

func initNewEncodeNone(t *testing.T) Encoder {
	newEncode, err := NewEncodeNone()
	require.Equal(t, err, nil)
	return newEncode
}

func TestEncodeNone(t *testing.T) {
	newEncode := initNewEncodeNone(t)
	encodeNone := newEncode.(*encodeNone)
	in := config.GenericMap{
		"varInt":    12,
		"varString": "string1",
		"varbool":   true,
	}
	encodeNone.Encode(in)
	require.Equal(t, in, encodeNone.prevRecord)
}
