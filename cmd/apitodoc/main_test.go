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

package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

type DocTags struct {
	Title  string            `yaml:"title" doc:"##title"`
	Field  string            `yaml:"field" doc:"field"`
	Slice  []string          `yaml:"slice" doc:"slice"`
	Map    map[string]string `yaml:"map" doc:"map"`
	Sub    DocSubTags        `yaml:"sub" doc:"sub"`
	SubPtr *DocSubTags       `yaml:"subPtr" doc:"subPtr"`
}

type DocSubTags struct {
	SubField string `yaml:"subField" doc:"subField"`
}

func Test_iterate(t *testing.T) {
	output := new(bytes.Buffer)
	expected := "\n##title\n<pre>\n title:\n</pre>     field: field\n     slice: slice\n     map: map\n     sub: sub\n         subField: subField\n     subPtr: subPtr\n         subField: subField\n"
	iterate(output, DocTags{}, 0)
	require.Equal(t, expected, output.String())
}

func Test_main(_ *testing.T) {
	main()
}
