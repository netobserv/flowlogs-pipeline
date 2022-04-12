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

package e2e

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/test/e2e"
	"sigs.k8s.io/e2e-framework/pkg/env"
)

var TestEnv env.Environment

func TestMain(m *testing.M) {
	yamlInfos := []e2e.YamlInfo{
		{
			YamlFile:    "k8s-objects.yaml",
			Namespace:   "test",
			PreCommands: []string{},
		},
	}
	e2e.Main(m, yamlInfos, &TestEnv)
}
