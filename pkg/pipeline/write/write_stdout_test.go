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

package write

import (
	"bufio"
	"os"
	"testing"

	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_WriteStdout(t *testing.T) {
	ws, err := NewWriteStdout()
	require.NoError(t, err)

	// Intercept standard output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	defer w.Close()
	defer r.Close()
	os.Stdout = w

	ws.Write([]config.GenericMap{{"key": "test"}})

	// read last line from standard output
	line, err := bufio.NewReader(r).ReadString('\n')
	require.NoError(t, err)
	os.Stdout = oldStdout

	assert.Contains(t, line, "map[key:test]")
}
