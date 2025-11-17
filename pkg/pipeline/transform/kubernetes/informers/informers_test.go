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

package informers

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/cni"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetInfo(t *testing.T) {
	metrics := operational.NewMetrics(&config.MetricsSettings{})
	kubeData := Informers{indexerHitMetric: metrics.CreateIndexerHitCounter()}
	pidx, hidx, sidx, ridx := SetupIndexerMocks(&kubeData)
	ridx.MockReplicaSet("rs1", "podNamespace", "dep1", "Deployment")
	ridx.FallbackNotFound()
	pidx.MockPod("1.2.3.4", "AA:BB:CC:DD:EE:FF", "eth0", "pod1", "podNamespace", "10.0.0.1", "pod1", "Pod")
	pidx.MockPod("1.2.3.5", "", "", "pod2", "podNamespace", "10.0.0.1", "rs1", "ReplicaSet")
	pidx.FallbackNotFound()
	sidx.MockService("1.2.3.100", "svc1", "svcNamespace")
	sidx.FallbackNotFound()
	hidx.MockNode("10.0.0.1", "node1")
	hidx.FallbackNotFound()

	// Test get orphan pod
	info := kubeData.IndexLookup(nil, "1.2.3.4")
	require.NotNil(t, info)

	pod1 := model.ResourceMetaData{
		Kind: "Pod",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "podNamespace",
		},
		HostName:         "node1",
		HostIP:           "10.0.0.1",
		OwnerName:        "pod1",
		OwnerKind:        "Pod",
		NetworkName:      "primary",
		IPs:              []string{"1.2.3.4"},
		SecondaryNetKeys: []string{"~~aa:bb:cc:dd:ee:ff"},
	}
	require.Equal(t, pod1, *info)

	// Test get same pod by mac
	info = kubeData.IndexLookup([]cni.SecondaryNetKey{{NetworkName: "custom-network", Key: "~~aa:bb:cc:dd:ee:ff"}}, "")
	require.NotNil(t, info)
	pod1.NetworkName = "custom-network"
	require.Equal(t, pod1, *info)

	// Test get pod owned
	info = kubeData.IndexLookup(nil, "1.2.3.5")
	require.NotNil(t, info)

	require.Equal(t, model.ResourceMetaData{
		Kind: "Pod",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod2",
			Namespace: "podNamespace",
		},
		HostName:    "node1",
		HostIP:      "10.0.0.1",
		OwnerName:   "dep1",
		OwnerKind:   "Deployment",
		NetworkName: "primary",
		IPs:         []string{"1.2.3.5"},
	}, *info)

	// Test get node
	info = kubeData.IndexLookup(nil, "10.0.0.1")
	require.NotNil(t, info)

	require.Equal(t, model.ResourceMetaData{
		Kind: "Node",
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
		OwnerName:   "node1",
		OwnerKind:   "Node",
		NetworkName: "primary",
		IPs:         []string{"10.0.0.1"},
	}, *info)

	// Test get service
	info = kubeData.IndexLookup(nil, "1.2.3.100")
	require.NotNil(t, info)

	require.Equal(t, model.ResourceMetaData{
		Kind: "Service",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc1",
			Namespace: "svcNamespace",
		},
		OwnerName:   "svc1",
		OwnerKind:   "Service",
		NetworkName: "primary",
		IPs:         []string{"1.2.3.100"},
	}, *info)

	// Test no match
	info = kubeData.IndexLookup(nil, "1.2.3.200")
	require.Nil(t, info)
}

// TestOwnershipTracking_GatewayAPI tests the ownership chain: Pod → ReplicaSet → Deployment → Gateway
func TestOwnershipTracking_GatewayAPI(t *testing.T) {
	metrics := operational.NewMetrics(&config.MetricsSettings{})
	kubeData := Informers{
		indexerHitMetric: metrics.CreateIndexerHitCounter(),
		config: Config{
			trackedKinds: []string{"Deployment", "Gateway"},
		},
	}

	pidx, hidx, sidx, ridx, didx := SetupIndexerMocksWithTrackedKinds(&kubeData, []string{"Deployment", "Gateway"})

	// Setup mocks for the ownership chain
	ridx.MockReplicaSet("rs1", "test-ns", "deploy1", "Deployment")
	ridx.FallbackNotFound()
	didx.MockDeployment("deploy1", "test-ns", "gateway1", "Gateway")

	pidx.MockPod("1.2.3.4", "", "", "pod1", "test-ns", "10.0.0.1", "rs1", "ReplicaSet")
	pidx.FallbackNotFound()
	hidx.MockNode("10.0.0.1", "node1")
	hidx.FallbackNotFound()
	sidx.FallbackNotFound()

	// Test: Pod should resolve to Gateway as final owner
	info := kubeData.IndexLookup(nil, "1.2.3.4")
	require.NotNil(t, info)
	require.Equal(t, "Gateway", info.OwnerKind)
	require.Equal(t, "gateway1", info.OwnerName)
}

// TestOwnershipTracking_OnlyDeployment tests when only Deployment is tracked (not Gateway)
func TestOwnershipTracking_OnlyDeployment(t *testing.T) {
	metrics := operational.NewMetrics(&config.MetricsSettings{})
	kubeData := Informers{
		indexerHitMetric: metrics.CreateIndexerHitCounter(),
		config: Config{
			trackedKinds: []string{"Deployment"}, // Gateway NOT tracked
		},
	}

	pidx, hidx, sidx, ridx, didx := SetupIndexerMocksWithTrackedKinds(&kubeData, []string{"Deployment"})

	ridx.MockReplicaSet("rs1", "test-ns", "deploy1", "Deployment")
	ridx.FallbackNotFound()
	didx.MockDeployment("deploy1", "test-ns", "gateway1", "Gateway")

	pidx.MockPod("1.2.3.4", "", "", "pod1", "test-ns", "10.0.0.1", "rs1", "ReplicaSet")
	pidx.FallbackNotFound()
	hidx.MockNode("10.0.0.1", "node1")
	hidx.FallbackNotFound()
	sidx.FallbackNotFound()

	// Test: Pod should resolve to Deployment (stops there because Gateway is not tracked)
	info := kubeData.IndexLookup(nil, "1.2.3.4")
	require.NotNil(t, info)
	require.Equal(t, "Deployment", info.OwnerKind)
	require.Equal(t, "deploy1", info.OwnerName)
}

// TestOwnershipTracking_NoTrackedKinds tests backward compatibility (no trackedKinds configured)
func TestOwnershipTracking_NoTrackedKinds(t *testing.T) {
	metrics := operational.NewMetrics(&config.MetricsSettings{})
	kubeData := Informers{
		indexerHitMetric: metrics.CreateIndexerHitCounter(),
		config: Config{
			trackedKinds: []string{}, // Empty list
		},
	}

	pidx, hidx, sidx, ridx := SetupIndexerMocks(&kubeData)

	ridx.MockReplicaSet("rs1", "test-ns", "deploy1", "Deployment")
	ridx.FallbackNotFound()

	pidx.MockPod("1.2.3.4", "", "", "pod1", "test-ns", "10.0.0.1", "rs1", "ReplicaSet")
	pidx.FallbackNotFound()
	hidx.MockNode("10.0.0.1", "node1")
	hidx.FallbackNotFound()
	sidx.FallbackNotFound()

	// Test: Pod should resolve to Deployment (ReplicaSet is always processed)
	info := kubeData.IndexLookup(nil, "1.2.3.4")
	require.NotNil(t, info)
	require.Equal(t, "Deployment", info.OwnerKind)
	require.Equal(t, "deploy1", info.OwnerName)
}

// TestOwnershipTracking_MaxDepth tests that ownership tracking stops at 3 levels
func TestOwnershipTracking_MaxDepth(t *testing.T) {
	metrics := operational.NewMetrics(&config.MetricsSettings{})
	kubeData := Informers{
		indexerHitMetric: metrics.CreateIndexerHitCounter(),
		config: Config{
			trackedKinds: []string{"Deployment", "Gateway"},
		},
	}

	pidx, hidx, sidx, ridx, didx := SetupIndexerMocksWithTrackedKinds(&kubeData, []string{"Deployment", "Gateway"})

	ridx.MockReplicaSet("rs1", "test-ns", "deploy1", "Deployment")
	ridx.FallbackNotFound()
	// Deployment owned by Gateway (which we can't traverse further without Gateway informer)
	didx.MockDeployment("deploy1", "test-ns", "gateway1", "Gateway")

	pidx.MockPod("1.2.3.4", "", "", "pod1", "test-ns", "10.0.0.1", "rs1", "ReplicaSet")
	pidx.FallbackNotFound()
	hidx.MockNode("10.0.0.1", "node1")
	hidx.FallbackNotFound()
	sidx.FallbackNotFound()

	// Test: Should stop at Gateway (3rd level), not continue to 4th level
	info := kubeData.IndexLookup(nil, "1.2.3.4")
	require.NotNil(t, info)
	require.Equal(t, "Gateway", info.OwnerKind)
	require.Equal(t, "gateway1", info.OwnerName)
}
