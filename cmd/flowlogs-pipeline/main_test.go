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

package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes"
)

func TestTheMain(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestTheMain")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()
	var castErr *exec.ExitError
	if errors.As(err, &castErr) && !castErr.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestPipelineConfigSetup(t *testing.T) {
	// Kube init mock
	kubernetes.MockInformers()

	js := `{
    "PipeLine": "[{\"name\":\"grpc\"},{\"follows\":\"grpc\",\"name\":\"enrich\"},{\"follows\":\"enrich\",\"name\":\"loki\"},{\"follows\":\"enrich\",\"name\":\"prometheus\"}]",
    "Parameters": "[{\"ingest\":{\"grpc\":{\"port\":2055},\"type\":\"grpc\"},\"name\":\"grpc\"},{\"name\":\"enrich\",\"transform\":{\"network\":{\"rules\":[{\"kubernetes\":{\"ipField\":\"SrcAddr\",\"output\":\"SrcK8S\"},\"type\":\"add_kubernetes\"},{\"kubernetes\":{\"ipField\":\"DstAddr\",\"output\":\"DstK8S\"},\"type\":\"add_kubernetes\"},{\"add_service\":{\"input\":\"DstPort\",\"output\":\"Service\",\"protocol\":\"Proto\"},\"type\":\"add_service\"},{\"add_subnet\":{\"input\":\"SrcAddr\",\"output\":\"SrcSubnet\",\"subnet_mask\":\"/16\"},\"type\":\"add_subnet\"}]},\"type\":\"network\"}},{\"name\":\"loki\",\"write\":{\"loki\":{\"batchSize\":102400,\"batchWait\":\"1s\",\"clientConfig\":{\"follow_redirects\":false,\"proxy_url\":null,\"tls_config\":{\"insecure_skip_verify\":false}},\"labels\":[\"SrcK8S_Namespace\",\"SrcK8S_OwnerName\",\"DstK8S_Namespace\",\"DstK8S_OwnerName\",\"FlowDirection\"],\"maxBackoff\":\"5m0s\",\"maxRetries\":10,\"minBackoff\":\"1s\",\"staticLabels\":{\"app\":\"netobserv-flowcollector\"},\"tenantID\":\"netobserv\",\"timeout\":\"10s\",\"timestampLabel\":\"TimeFlowEndMs\",\"timestampScale\":\"1ms\",\"url\":\"http://loki.netobserv.svc:3100/\"},\"type\":\"loki\"}},{\"encode\":{\"prom\":{\"metrics\":[{\"buckets\":null,\"labels\":[\"Service\",\"SrcK8S_Namespace\"],\"name\":\"bandwidth_per_network_service_per_namespace\",\"type\":\"counter\",\"valueKey\":\"Bytes\"},{\"buckets\":null,\"labels\":[\"SrcSubnet\"],\"name\":\"bandwidth_per_source_subnet\",\"type\":\"counter\",\"valueKey\":\"Bytes\"},{\"buckets\":null,\"labels\":[\"Service\"],\"name\":\"network_service_total\",\"type\":\"counter\",\"valueKey\":\"\"}],\"prefix\":\"netobserv_\"},\"type\":\"prom\"},\"name\":\"prometheus\"}]",
    "Health": {
        "Port": "8080"
    },
    "Profile": {
        "Port": 0
    }
}`
	var opts config.Options
	err := json.Unmarshal([]byte(js), &opts)
	require.NoError(t, err)
	cfg, err := config.ParseConfig(&opts)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	mainPipeline, err := pipeline.NewPipeline(&cfg)
	require.NoError(t, err)
	require.NotNil(t, mainPipeline)
}
