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
	"context"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/test/e2e"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var TestEnv env.Environment

var manifestDeployDefinitions = e2e.ManifestDeployDefinitions{
	e2e.ManifestDeployDefinition{
		YamlFile: "strimzi-cluster-operator-0.31.0.yaml",
		PostFunction: func(_ context.Context, _ *envconf.Config, _ string) error {
			// Wait few seconds to allow the CRD to be registered so the next step does not fail
			time.Sleep(5 * time.Second)
			return nil
		},
	},
	e2e.ManifestDeployDefinition{
		YamlFile:     "kafka.strimzi.yaml",
		PostFunction: postStrimziDeploy,
	},
	e2e.ManifestDeployDefinition{
		YamlFile: "flp-config.yaml",
	},
	e2e.ManifestDeployDefinition{
		YamlFile:     "flp.yaml",
		PostFunction: postFLPDeploy,
	},
}

func TestMain(m *testing.M) {
	e2e.Main(m, manifestDeployDefinitions, &TestEnv)
}
