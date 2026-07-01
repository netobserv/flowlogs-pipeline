package k8scache

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/datasource"
	inf "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/informers"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test data - same pattern as enrich_test.go
var testIPInfo = map[string]*model.ResourceMetaData{
	"10.0.0.1": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "test-ns-1",
			UID:       "pod-uid-1",
			Labels: map[string]string{
				"app":     "web",
				"version": "v1",
			},
			Annotations: map[string]string{
				"description": "test pod",
			},
		},
		Kind:        "Pod",
		OwnerName:   "test-deployment",
		OwnerKind:   "Deployment",
		HostName:    "node-1",
		HostIP:      "192.168.1.1",
		NetworkName: "primary",
		IPs:         []string{"10.0.0.1"},
	},
	"10.0.0.2": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "test-ns-2",
			UID:       "pod-uid-2",
		},
		Kind:        "Pod",
		OwnerName:   "test-statefulset",
		OwnerKind:   "StatefulSet",
		HostName:    "node-2",
		HostIP:      "192.168.1.2",
		NetworkName: "primary",
		IPs:         []string{"10.0.0.2"},
	},
	"192.168.1.1": {
		ObjectMeta: v1.ObjectMeta{
			Name: "node-1",
			UID:  "node-uid-1",
		},
		Kind:      "Node",
		OwnerName: "node-1",
		OwnerKind: "Node",
		IPs:       []string{"192.168.1.1"},
	},
}

var testNodes = map[string]*model.ResourceMetaData{
	"node-1": {
		ObjectMeta: v1.ObjectMeta{
			Name: "node-1",
		},
		Kind: "Node",
	},
}

func setupTestDatasource() *datasource.Datasource {
	_, informers := inf.SetupStubs(testIPInfo, nil, testNodes)
	return &datasource.Datasource{Informers: informers}
}

func setupTestDatasourceWithStore() *datasource.Datasource {
	_, informers := inf.SetupStubs(testIPInfo, nil, testNodes)
	ds := &datasource.Datasource{Informers: informers}
	ds.SetKubernetesStore(datasource.NewKubernetesStore())
	return ds
}

// TestBackwardCompatibility ensures existing datasource functionality is not broken
func TestBackwardCompatibility_DatasourceLookup(t *testing.T) {
	ds := setupTestDatasource()

	// Test IP lookup - existing functionality
	result := ds.IndexLookup(nil, "10.0.0.1")
	require.NotNil(t, result)
	assert.Equal(t, "test-pod-1", result.Name)
	assert.Equal(t, "test-ns-1", result.Namespace)
	assert.Equal(t, "Pod", result.Kind)

	// Test node lookup - existing functionality
	node, err := ds.GetNodeByName("node-1")
	require.NoError(t, err)
	require.NotNil(t, node)
	assert.Equal(t, "node-1", node.Name)
}

// TestKubernetesCacheServer_Creation tests server instantiation
func TestKubernetesCacheServer_Creation(t *testing.T) {
	ds := setupTestDatasource()

	server := NewKubernetesCacheServer(ds)

	require.NotNil(t, server)
	assert.NotNil(t, server.datasource)
}

// TestKubernetesCacheServer_ReceivesAdd tests that when a client sends an ADD,
// the server processes it correctly
func TestKubernetesCacheServer_ReceivesAdd(t *testing.T) {
	ds := setupTestDatasource()
	// Attach KubernetesStore so updates are actually stored
	ds.SetKubernetesStore(datasource.NewKubernetesStore())
	server := NewKubernetesCacheServer(ds)

	// Create mock stream
	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	// Client (informers) sends ADD update
	addUpdate := &CacheUpdate{
		Version:    1,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "test-ns-1",
				Name:      "test-pod-1",
				Uid:       "pod-uid-1",
				Ips:       []string{"10.0.0.1"},
			},
		},
	}
	mockStream.sendChan <- addUpdate
	close(mockStream.sendChan)

	// Run server
	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	// Verify server sent SyncRequest first
	require.Greater(t, len(mockStream.recvMsgs), 0, "Should have sent at least SyncRequest")
	firstMsg := mockStream.recvMsgs[0]
	req, ok := firstMsg.Message.(*SyncMessage_Request)
	require.True(t, ok, "First message should be SyncRequest")
	assert.NotEmpty(t, req.Request.ProcessorId)

	// Verify server sent ACK
	require.Greater(t, len(mockStream.recvMsgs), 1, "Should have sent ACK")
	ackMsg := mockStream.recvMsgs[1]
	ack, ok := ackMsg.Message.(*SyncMessage_Ack)
	require.True(t, ok, "Second message should be ACK")
	assert.True(t, ack.Ack.Success)
	assert.Equal(t, int64(1), ack.Ack.Version)

	// Verify resource was added to store
	meta := ds.IndexLookup(nil, "10.0.0.1")
	require.NotNil(t, meta, "Resource should be in store")
	assert.Equal(t, "test-pod-1", meta.Name)
}

// TestKubernetesCacheServer_ReceivesIncrementalUpdate tests incremental updates
func TestKubernetesCacheServer_ReceivesIncrementalUpdate(t *testing.T) {
	ds := setupTestDatasource()
	server := NewKubernetesCacheServer(ds)

	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	// Send incremental update
	update := &CacheUpdate{
		Version:    2,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "new-pod",
			},
		},
	}
	mockStream.sendChan <- update
	close(mockStream.sendChan)

	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	// Verify ACK was sent
	require.Greater(t, len(mockStream.recvMsgs), 1)
	ackMsg := mockStream.recvMsgs[1]
	ack, ok := ackMsg.Message.(*SyncMessage_Ack)
	require.True(t, ok)
	assert.True(t, ack.Ack.Success)
	assert.Equal(t, int64(2), ack.Ack.Version)
}

// TestKubernetesCacheServer_MultipleUpdates tests receiving multiple updates in sequence
func TestKubernetesCacheServer_MultipleUpdates(t *testing.T) {
	ds := setupTestDatasource()
	server := NewKubernetesCacheServer(ds)

	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	// Send ADD
	mockStream.sendChan <- &CacheUpdate{
		Version:    1,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries:    []*ResourceEntry{{Kind: "Pod", Name: "pod1", Namespace: "default"}},
	}

	// Send another ADD
	mockStream.sendChan <- &CacheUpdate{
		Version:    2,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries:    []*ResourceEntry{{Kind: "Pod", Name: "pod2", Namespace: "default"}},
	}

	// Send DELETE
	mockStream.sendChan <- &CacheUpdate{
		Version:    3,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_DELETE,
		Entries:    []*ResourceEntry{{Kind: "Pod", Name: "pod1", Namespace: "default"}},
	}

	close(mockStream.sendChan)

	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	// Should have: 1 SyncRequest + 3 ACKs
	assert.Equal(t, 4, len(mockStream.recvMsgs))

	// Verify version tracking
	assert.Equal(t, int64(3), server.GetCurrentVersion())
}

// TestKubernetesCacheServer_ErrorHandling tests error scenarios
func TestKubernetesCacheServer_ErrorHandling(t *testing.T) {
	ds := setupTestDatasource()
	server := NewKubernetesCacheServer(ds)

	t.Run("client disconnects abruptly", func(t *testing.T) {
		mockStream := &mockStreamServer{
			ctx:       context.Background(),
			sendChan:  make(chan *CacheUpdate, 10),
			recvMsgs:  make([]*SyncMessage, 0),
			firstSend: true,
		}

		// Close immediately
		close(mockStream.sendChan)

		// Should handle gracefully
		err := server.StreamUpdates(mockStream)
		assert.NoError(t, err) // EOF is normal when client disconnects
	})
}

// TestKubernetesCacheServer_WithKubernetesStore tests operations using KubernetesStore
func TestKubernetesCacheServer_WithKubernetesStore(t *testing.T) {
	ds := setupTestDatasourceWithStore()
	server := NewKubernetesCacheServer(ds)

	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	// Send ADD
	mockStream.sendChan <- &CacheUpdate{
		Version:    1,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "test-ns",
				Name:      "store-pod-1",
				Uid:       "store-pod-uid-1",
				Ips:       []string{"10.0.1.1"},
				Labels: map[string]string{
					"app": "test",
				},
			},
		},
	}

	// Send UPDATE
	mockStream.sendChan <- &CacheUpdate{
		Version:    2,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_UPDATE,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "test-ns",
				Name:      "store-pod-1",
				Uid:       "store-pod-uid-1",
				Ips:       []string{"10.0.1.1"},
				Labels: map[string]string{
					"app":     "test",
					"version": "v2", // Updated label
				},
			},
		},
	}

	close(mockStream.sendChan)

	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	// Verify resource was added and updated in store
	meta := ds.IndexLookup(nil, "10.0.1.1")
	require.NotNil(t, meta, "Resource should be in KubernetesStore")
	assert.Equal(t, "store-pod-1", meta.Name)
	assert.Equal(t, "test-ns", meta.Namespace)
	assert.Equal(t, "v2", meta.Labels["version"], "Label should be updated")

	// Verify ACKs were sent
	require.Equal(t, 3, len(mockStream.recvMsgs)) // SyncRequest + 2 ACKs
}

// TestKubernetesCacheServer_DeleteFromStore tests DELETE operation on KubernetesStore
func TestKubernetesCacheServer_DeleteFromStore(t *testing.T) {
	ds := setupTestDatasourceWithStore()
	server := NewKubernetesCacheServer(ds)

	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	// First ADD a resource
	mockStream.sendChan <- &CacheUpdate{
		Version:    1,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "test-ns",
				Name:      "delete-me",
				Uid:       "delete-pod-uid",
				Ips:       []string{"10.0.2.1"},
			},
		},
	}

	// Then DELETE it
	mockStream.sendChan <- &CacheUpdate{
		Version:    2,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_DELETE,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "test-ns",
				Name:      "delete-me",
			},
		},
	}

	close(mockStream.sendChan)

	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	// Verify resource was deleted from store
	meta := ds.IndexLookup(nil, "10.0.2.1")
	assert.Nil(t, meta, "Resource should be deleted from KubernetesStore")
}

// TestKubernetesCacheServer_StoreReplacesInformers tests that KubernetesStore replaces Informers
func TestKubernetesCacheServer_StoreReplacesInformers(t *testing.T) {
	ds := setupTestDatasourceWithStore()
	server := NewKubernetesCacheServer(ds)

	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	// Add resource via gRPC (goes to KubernetesStore)
	mockStream.sendChan <- &CacheUpdate{
		Version:    1,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "grpc-ns",
				Name:      "grpc-pod",
				Ips:       []string{"10.0.3.1"},
			},
		},
	}

	close(mockStream.sendChan)

	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	// Should find resource from KubernetesStore
	grpcMeta := ds.IndexLookup(nil, "10.0.3.1")
	require.NotNil(t, grpcMeta)
	assert.Equal(t, "grpc-pod", grpcMeta.Name)

	// When KubernetesStore is set, Informers are bypassed
	// (testIPInfo from setupTestDatasourceWithStore is in Informers, not in Store)
	informerMeta := ds.IndexLookup(nil, "10.0.0.1")
	assert.Nil(t, informerMeta, "KubernetesStore replaces Informers, so Informer data is not accessible")
}

// TestKubernetesCacheServer_FallbackToInformers verifies fallback when Store is not set
func TestKubernetesCacheServer_FallbackToInformers(t *testing.T) {
	// Datasource WITHOUT KubernetesStore - falls back to Informers
	ds := setupTestDatasource()

	// Should find resource from Informers (set up in testIPInfo)
	informerMeta := ds.IndexLookup(nil, "10.0.0.1")
	require.NotNil(t, informerMeta, "Should fallback to Informers when Store is not set")
	assert.Equal(t, "test-pod-1", informerMeta.Name)
}

// TestKubernetesCacheServer_SnapshotReplace tests that the server properly handles
// a full snapshot (is_snapshot=true) by calling Replace() instead of AddOrUpdate()
func TestKubernetesCacheServer_SnapshotReplace(t *testing.T) {
	ds := setupTestDatasourceWithStore()
	server := NewKubernetesCacheServer(ds)

	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	// First, send a snapshot with initial data
	mockStream.sendChan <- &CacheUpdate{
		Version:    1,
		IsSnapshot: true, // This is a full snapshot
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "test-ns",
				Name:      "snapshot-pod-1",
				Uid:       "snapshot-pod-uid-1",
				Ips:       []string{"10.0.10.1"},
			},
			{
				Kind:      "Pod",
				Namespace: "test-ns",
				Name:      "snapshot-pod-2",
				Uid:       "snapshot-pod-uid-2",
				Ips:       []string{"10.0.10.2"},
			},
		},
	}

	// Then send another snapshot that should replace the entire store
	mockStream.sendChan <- &CacheUpdate{
		Version:    2,
		IsSnapshot: true, // This is a full snapshot
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "test-ns",
				Name:      "snapshot-pod-3",
				Uid:       "snapshot-pod-uid-3",
				Ips:       []string{"10.0.10.3"},
			},
		},
	}

	close(mockStream.sendChan)

	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	// Verify first snapshot was replaced by second snapshot
	// The first two pods should NOT be in the store
	meta1 := ds.IndexLookup(nil, "10.0.10.1")
	assert.Nil(t, meta1, "First snapshot pod should be replaced")

	meta2 := ds.IndexLookup(nil, "10.0.10.2")
	assert.Nil(t, meta2, "First snapshot pod should be replaced")

	// Only the third pod from the second snapshot should exist
	meta3 := ds.IndexLookup(nil, "10.0.10.3")
	require.NotNil(t, meta3, "Second snapshot pod should exist")
	assert.Equal(t, "snapshot-pod-3", meta3.Name)

	// Verify ACKs were sent for both snapshots
	require.Equal(t, 3, len(mockStream.recvMsgs)) // SyncRequest + 2 ACKs
}

// TestKubernetesCacheServer_MultiBatchSnapshot verifies that a snapshot split across
// multiple batches (first batch Replace, rest ADD) retains all entries in the store.
func TestKubernetesCacheServer_MultiBatchSnapshot(t *testing.T) {
	ds := setupTestDatasourceWithStore()
	server := NewKubernetesCacheServer(ds)

	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	const batchSize = 100
	const totalEntries = 150

	firstBatch := make([]*ResourceEntry, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		firstBatch = append(firstBatch, &ResourceEntry{
			Kind:      "Pod",
			Namespace: "test-ns",
			Name:      fmt.Sprintf("pod-%d", i),
			Uid:       fmt.Sprintf("uid-%d", i),
			Ips:       []string{fmt.Sprintf("10.1.%d.1", i)},
		})
	}
	mockStream.sendChan <- &CacheUpdate{
		Version:    1,
		IsSnapshot: true,
		Operation:  OperationType_OPERATION_ADD,
		Entries:    firstBatch,
	}

	secondBatch := make([]*ResourceEntry, 0, totalEntries-batchSize)
	for i := batchSize; i < totalEntries; i++ {
		secondBatch = append(secondBatch, &ResourceEntry{
			Kind:      "Pod",
			Namespace: "test-ns",
			Name:      fmt.Sprintf("pod-%d", i),
			Uid:       fmt.Sprintf("uid-%d", i),
			Ips:       []string{fmt.Sprintf("10.1.%d.1", i)},
		})
	}
	mockStream.sendChan <- &CacheUpdate{
		Version:    2,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries:    secondBatch,
	}

	close(mockStream.sendChan)

	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	for i := 0; i < totalEntries; i++ {
		meta := ds.IndexLookup(nil, fmt.Sprintf("10.1.%d.1", i))
		require.NotNil(t, meta, "entry %d should be present after multi-batch snapshot", i)
	}
}

// TestKubernetesCacheServer_InitialSyncWithSnapshot tests the typical scenario where
// a fresh processor (LastVersion=0) receives a full snapshot from the client
func TestKubernetesCacheServer_InitialSyncWithSnapshot(t *testing.T) {
	ds := setupTestDatasourceWithStore()
	server := NewKubernetesCacheServer(ds)

	mockStream := &mockStreamServer{
		ctx:       context.Background(),
		sendChan:  make(chan *CacheUpdate, 10),
		recvMsgs:  make([]*SyncMessage, 0),
		firstSend: true,
	}

	// Server should send SyncRequest with LastVersion=0
	// Client responds with a full snapshot
	mockStream.sendChan <- &CacheUpdate{
		Version:    100, // Some arbitrary version number from the client
		IsSnapshot: true,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "kube-system",
				Name:      "coredns-1",
				Ips:       []string{"10.96.0.10"},
			},
			{
				Kind:      "Node",
				Namespace: "",
				Name:      "worker-1",
				Ips:       []string{"192.168.1.10"},
			},
		},
	}

	// After the snapshot, incremental updates follow
	mockStream.sendChan <- &CacheUpdate{
		Version:    101,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "kube-system",
				Name:      "coredns-2",
				Ips:       []string{"10.96.0.11"},
			},
		},
	}

	close(mockStream.sendChan)

	err := server.StreamUpdates(mockStream)
	require.NoError(t, err)

	// Verify SyncRequest was sent with LastVersion=0
	require.Greater(t, len(mockStream.recvMsgs), 0)
	firstMsg := mockStream.recvMsgs[0]
	req, ok := firstMsg.Message.(*SyncMessage_Request)
	require.True(t, ok)
	assert.Equal(t, int64(0), req.Request.LastVersion, "Initial sync should request from version 0")

	// Verify both pods are in the store (snapshot + incremental)
	pod1 := ds.IndexLookup(nil, "10.96.0.10")
	require.NotNil(t, pod1)
	assert.Equal(t, "coredns-1", pod1.Name)

	pod2 := ds.IndexLookup(nil, "10.96.0.11")
	require.NotNil(t, pod2)
	assert.Equal(t, "coredns-2", pod2.Name)

	// Verify node is in the store
	node, err := ds.GetNodeByName("worker-1")
	require.NoError(t, err)
	require.NotNil(t, node)
	assert.Equal(t, "worker-1", node.Name)

	// Verify version was updated to the latest
	assert.Equal(t, int64(101), server.GetCurrentVersion())
}

// mockStreamServer implements the server-side stream for testing
// Note: With the corrected protocol, the server:
// - Receives CacheUpdate (from client)
// - Sends SyncMessage (to client)
type mockStreamServer struct {
	grpc.ServerStream
	ctx       context.Context
	sendChan  chan *CacheUpdate // What client sends
	recvMsgs  []*SyncMessage    // What server sent
	firstSend bool
}

func (m *mockStreamServer) Context() context.Context {
	return m.ctx
}

// Send is called by the server to send SyncMessage to client
func (m *mockStreamServer) Send(msg *SyncMessage) error {
	m.recvMsgs = append(m.recvMsgs, msg)
	return nil
}

// Recv is called by the server to receive CacheUpdate from client
func (m *mockStreamServer) Recv() (*CacheUpdate, error) {
	update, ok := <-m.sendChan
	if !ok {
		return nil, io.EOF
	}
	return update, nil
}
