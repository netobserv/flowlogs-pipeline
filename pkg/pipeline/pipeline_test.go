/*
 * Copyright (C) 2019 IBM, Inc.
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

package pipeline

import (
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/write"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_transformToLoki(t *testing.T) {
	var transformed []config.GenericMap
	input := config.GenericMap{"key": "value"}
	transform, err := transform.NewTransformNone()
	require.NoError(t, err)
	transformed = append(transformed, transform.Transform(input))

	config.Opt.PipeLine.Write.Loki = "{}"
	loki, err := write.NewWriteLoki()
	loki.Write(transformed)
	require.NoError(t, err)
}
