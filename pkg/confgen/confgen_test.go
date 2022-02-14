/*
 * Copyright (C) 2021 IBM, Inc.
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

package confgen

import (
	"github.com/stretchr/testify/require"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func getConfGen() *ConfGen {
	return &ConfGen{}
}

func Test_checkHeader(t *testing.T) {
	filename := "/tmp/header.check.txt"
	fakeFilename := "/tmp/fake_file.does.exist"
	wrongHeader := "#wrong_confgen"
	cg := getConfGen()
	err := cg.checkHeader(fakeFilename)
	require.Error(t, err)

	err = os.WriteFile(filename, []byte(wrongHeader), 0644)
	require.NoError(t, err)
	err = cg.checkHeader(filename)
	require.Error(t, err)

	err = os.WriteFile(filename, []byte(definitionHeader), 0644)
	require.NoError(t, err)
	err = cg.checkHeader(filename)
	require.NoError(t, err)
}

const networkDefinitionConfiguration = `#fl2m_confgen
description:
  test description
details:
  test details
usage:
  test usage
labels:
  - test
  - label
transform:
  rules:
    - input: testInput
      output: testOutput
      type: add_service
      parameters: proto
extract:
  aggregates:
    - name: test_aggregates
      by:
        - service
      operation: sum
      recordKey: test_record_key
encode:
  type: prom
  prom:
    metrics:
      - name: test_metric
        type: gauge
        valuekey: test_aggregates_value
        labels:
          - by
          - aggregate
visualization:
  type: grafana
  grafana:
    - expr: 'test expression'
      type: graphPanel
      dashboard: test
      title:
        Test grafana title
`

func Test_parseFile(t *testing.T) {
	fakeFilename := "/tmp/fake_file.does.exist"
	filename := "/tmp/parse_file.check.txt"
	cg := getConfGen()
	err := cg.parseFile(fakeFilename)
	require.Error(t, err)

	err = os.WriteFile(filename, []byte(networkDefinitionConfiguration), 0644)
	require.NoError(t, err)
	err = cg.parseFile(filename)
	require.NoError(t, err)
}

func Test_getDefinitionFiles(t *testing.T) {
	dirPath := "/tmp/getDefinitionFilesTest"
	filename := "/def.yaml"
	cg := getConfGen()
	err := os.MkdirAll(dirPath, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dirPath, filename), []byte(networkDefinitionConfiguration), 0644)
	require.NoError(t, err)
	files := cg.getDefinitionFiles(dirPath)
	require.Equal(t, 1, len(files))
	expected := []string{path.Join(dirPath, filename)}
	require.ElementsMatch(t, expected, files)
}

func Test_NewConfGen(t *testing.T) {
	_, err := NewConfGen()
	require.NoError(t, err)
}
