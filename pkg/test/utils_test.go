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

package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_GetIngestMockEntry(t *testing.T) {
	entry := GetIngestMockEntry(true)
	require.Equal(t, entry, GetIngestMockEntry(true))

	entry = GetIngestMockEntry(false)
	require.Equal(t, entry, GetIngestMockEntry(false))

	entry = GetIngestMockEntry(true)
	require.NotEqual(t, entry, GetIngestMockEntry(false))
}

func Test_InitConfig(t *testing.T) {
	viper, out := InitConfig(t, "")
	require.NotNil(t, viper)
	require.NotNil(t, out)
}

func Test_GetExtractMockEntry(t *testing.T) {
	entry := GetExtractMockEntry()
	require.Equal(t, entry, GetExtractMockEntry())
}
