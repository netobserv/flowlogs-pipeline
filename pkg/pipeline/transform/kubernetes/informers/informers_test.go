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
	pidx.MockPod("1.2.3.4", "pod1", "podNamespace", "10.0.0.1", "pod1", "Pod", &cni.NetStatItem{Name: "custom-network", MAC: "AA:BB:CC:DD:EE:FF", Interface: "eth0"})
	pidx.MockPod("1.2.3.5", "pod2", "podNamespace", "10.0.0.1", "rs1", "ReplicaSet", nil)
	pidx.FallbackNotFound()
	sidx.MockService("1.2.3.100", "svc1", "svcNamespace")
	sidx.FallbackNotFound()
	hidx.MockNode("10.0.0.1", "node1")
	hidx.FallbackNotFound()

	// Test get orphan pod
	info := kubeData.IndexLookup(nil, "1.2.3.4")
	require.NotNil(t, info)

	expected := model.ResourceMetaData{
		Kind: "Pod",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "podNamespace",
		},
		HostName:          "node1",
		HostIP:            "10.0.0.1",
		OwnerName:         "pod1",
		OwnerKind:         "Pod",
		NetworkName:       "primary",
		IPs:               []string{"1.2.3.4"},
		SecondaryNetKeys:  []string{"~~aa:bb:cc:dd:ee:ff"},
		SecondaryNetNames: map[string]string{"~~aa:bb:cc:dd:ee:ff": "custom-network"},
	}
	require.Equal(t, expected, *info)

	// Test get same pod by mac
	info = kubeData.IndexLookup([]string{"~~aa:bb:cc:dd:ee:ff"}, "")
	require.NotNil(t, info)
	expected.NetworkName = "custom-network"
	require.Equal(t, expected, *info)

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

// TestGetInfo_TerminatedPodContention checks that a live Pod wins a reused IP over a
// terminated one, while a terminated Pod is still returned when no live Pod owns the IP.
func TestGetInfo_TerminatedPodContention(t *testing.T) {
	metrics := operational.NewMetrics(&config.MetricsSettings{})
	kubeData := Informers{indexerHitMetric: metrics.CreateIndexerHitCounter()}
	pidx, hidx, sidx, ridx := SetupIndexerMocks(&kubeData)
	ridx.FallbackNotFound()

	// A completed pod (e.g. TaskRun) and a running pod both hold 1.2.3.4;
	// the terminated one is returned first by the index.
	pidx.MockPodsForIP("1.2.3.4",
		model.ResourceMetaData{
			ObjectMeta: metav1.ObjectMeta{Name: "old-taskrun-pod", Namespace: "podNamespace"},
			OwnerName:  "old-taskrun-pod", OwnerKind: "Pod", HostIP: "10.0.0.1", Terminated: true,
		},
		model.ResourceMetaData{
			ObjectMeta: metav1.ObjectMeta{Name: "running-pod", Namespace: "podNamespace"},
			OwnerName:  "running-pod", OwnerKind: "Pod", HostIP: "10.0.0.1",
		},
	)
	// A terminated pod with no live contender still keeps its IP.
	pidx.MockPodsForIP("1.2.3.5",
		model.ResourceMetaData{
			ObjectMeta: metav1.ObjectMeta{Name: "lonely-terminated-pod", Namespace: "podNamespace"},
			OwnerName:  "lonely-terminated-pod", OwnerKind: "Pod", HostIP: "10.0.0.1", Terminated: true,
		},
	)
	pidx.FallbackNotFound()
	sidx.FallbackNotFound()
	hidx.MockNode("10.0.0.1", "node1")
	hidx.FallbackNotFound()

	// Live pod wins the reused IP
	info := kubeData.IndexLookup(nil, "1.2.3.4")
	require.NotNil(t, info)
	require.Equal(t, "running-pod", info.Name)
	require.False(t, info.Terminated)

	// Terminated pod is still returned when it is the only match
	info = kubeData.IndexLookup(nil, "1.2.3.5")
	require.NotNil(t, info)
	require.Equal(t, "lonely-terminated-pod", info.Name)
	require.True(t, info.Terminated)
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

	pidx.MockPod("1.2.3.4", "pod1", "test-ns", "10.0.0.1", "rs1", "ReplicaSet", nil)
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

	pidx.MockPod("1.2.3.4", "pod1", "test-ns", "10.0.0.1", "rs1", "ReplicaSet", nil)
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

	pidx.MockPod("1.2.3.4", "pod1", "test-ns", "10.0.0.1", "rs1", "ReplicaSet", nil)
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

	pidx.MockPod("1.2.3.4", "pod1", "test-ns", "10.0.0.1", "rs1", "ReplicaSet", nil)
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

func TestStop(t *testing.T) {
	inf := &Informers{}
	inf.stopChan = make(chan struct{})
	inf.mdStopChan = make(chan struct{})

	// Test calling Stop closes both channels
	inf.Stop()

	// Verify stopChan is closed
	select {
	case <-inf.stopChan:
		// Channel is closed, good
	default:
		t.Fatal("stopChan should be closed after Stop()")
	}

	// Verify mdStopChan is closed
	select {
	case <-inf.mdStopChan:
		// Channel is closed, good
	default:
		t.Fatal("mdStopChan should be closed after Stop()")
	}

	// Test calling Stop again is idempotent (doesn't panic)
	inf.Stop()
}
