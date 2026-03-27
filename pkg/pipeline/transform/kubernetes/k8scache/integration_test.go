package k8scache

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/datasource"
	inf "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/informers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestIntegration_ServerReceivesAdd tests that the server
// can receive an ADD update from a real gRPC client
func TestIntegration_ServerReceivesAdd(t *testing.T) {
	// Setup test datasource
	_, informers := inf.SetupStubs(testIPInfo, nil, testNodes)
	ds := &datasource.Datasource{Informers: informers}
	ds.SetKubernetesStore(datasource.NewKubernetesStore())

	// Create cache server
	cacheServer := NewKubernetesCacheServer(ds)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	RegisterKubernetesCacheServiceServer(grpcServer, cacheServer)

	// Start server on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	address := listener.Addr().String()

	// Start server in background
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Failed to connect to server")
	defer conn.Close()

	// Create client
	client := NewKubernetesCacheServiceClient(conn)

	// Open bidirectional stream
	stream, err := client.StreamUpdates(ctx)
	require.NoError(t, err)

	// Client should receive SyncRequest from server first
	syncMsg, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, syncMsg)

	req, ok := syncMsg.Message.(*SyncMessage_Request)
	require.True(t, ok, "Expected SyncRequest from server")
	assert.Equal(t, int64(0), req.Request.LastVersion)

	// Client sends ADD update
	addUpdate := &CacheUpdate{
		Version:    1,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries: []*ResourceEntry{
			{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				Uid:       "test-uid",
				Ips:       []string{"10.0.0.100"},
			},
		},
	}

	err = stream.Send(addUpdate)
	require.NoError(t, err)

	// Client should receive ACK from server
	ackMsg, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, ackMsg)

	ack, ok := ackMsg.Message.(*SyncMessage_Ack)
	require.True(t, ok, "Expected SyncAck from server")
	assert.True(t, ack.Ack.Success, "Server should ACK successfully")
	assert.Equal(t, int64(1), ack.Ack.Version)

	// Close stream
	err = stream.CloseSend()
	assert.NoError(t, err)

	fmt.Printf("✓ Integration test passed: server on %s received and acknowledged ADD update\n", address)
}

// TestIntegration_MultipleUpdatesFlow tests a realistic update flow
func TestIntegration_MultipleUpdatesFlow(t *testing.T) {
	// Setup
	_, informers := inf.SetupStubs(testIPInfo, nil, testNodes)
	ds := &datasource.Datasource{Informers: informers}
	cacheServer := NewKubernetesCacheServer(ds)

	grpcServer := grpc.NewServer()
	RegisterKubernetesCacheServiceServer(grpcServer, cacheServer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	address := listener.Addr().String()

	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := NewKubernetesCacheServiceClient(conn)
	stream, err := client.StreamUpdates(ctx)
	require.NoError(t, err)

	// Receive initial SyncRequest
	syncMsg, err := stream.Recv()
	require.NoError(t, err)
	_, ok := syncMsg.Message.(*SyncMessage_Request)
	require.True(t, ok)

	// Send first ADD
	err = stream.Send(&CacheUpdate{
		Version:    1,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries: []*ResourceEntry{
			{Kind: "Pod", Name: "pod1", Namespace: "default"},
		},
	})
	require.NoError(t, err)

	// Receive ACK
	ackMsg, err := stream.Recv()
	require.NoError(t, err)
	ack, ok := ackMsg.Message.(*SyncMessage_Ack)
	require.True(t, ok)
	assert.True(t, ack.Ack.Success)
	assert.Equal(t, int64(1), ack.Ack.Version)

	// Send second ADD
	err = stream.Send(&CacheUpdate{
		Version:    2,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries:    []*ResourceEntry{{Kind: "Pod", Name: "pod2", Namespace: "default"}},
	})
	require.NoError(t, err)

	// Receive ACK
	ackMsg, err = stream.Recv()
	require.NoError(t, err)
	ack, ok = ackMsg.Message.(*SyncMessage_Ack)
	require.True(t, ok)
	assert.True(t, ack.Ack.Success)
	assert.Equal(t, int64(2), ack.Ack.Version)

	// Send DELETE
	err = stream.Send(&CacheUpdate{
		Version:    3,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_DELETE,
		Entries:    []*ResourceEntry{{Kind: "Pod", Name: "pod1", Namespace: "default"}},
	})
	require.NoError(t, err)

	// Receive ACK
	ackMsg, err = stream.Recv()
	require.NoError(t, err)
	ack, ok = ackMsg.Message.(*SyncMessage_Ack)
	require.True(t, ok)
	assert.True(t, ack.Ack.Success)
	assert.Equal(t, int64(3), ack.Ack.Version)

	// Verify server state
	assert.Equal(t, int64(3), cacheServer.GetCurrentVersion())

	err = stream.CloseSend()
	assert.NoError(t, err)

	fmt.Printf("✓ Integration test passed: full update flow (ADD + ADD + DELETE)\n")
}

// TestIntegration_MultipleClientsConnect tests that multiple informer clients
// can connect to the same server
func TestIntegration_MultipleClientsConnect(t *testing.T) {
	// Setup
	_, informers := inf.SetupStubs(testIPInfo, nil, testNodes)
	ds := &datasource.Datasource{Informers: informers}
	cacheServer := NewKubernetesCacheServer(ds)

	grpcServer := grpc.NewServer()
	RegisterKubernetesCacheServiceServer(grpcServer, cacheServer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	address := listener.Addr().String()

	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create 3 clients concurrently
	numClients := 3
	done := make(chan bool, numClients)
	errors := make(chan error, numClients)

	for i := range numClients {
		go func(clientID int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			conn, err := grpc.NewClient(address,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				errors <- fmt.Errorf("client %d failed to connect: %w", clientID, err)
				return
			}
			defer conn.Close()

			client := NewKubernetesCacheServiceClient(conn)
			stream, err := client.StreamUpdates(ctx)
			if err != nil {
				errors <- fmt.Errorf("client %d failed to open stream: %w", clientID, err)
				return
			}

			// Receive SyncRequest
			syncMsg, err := stream.Recv()
			if err != nil {
				errors <- fmt.Errorf("client %d failed to receive SyncRequest: %w", clientID, err)
				return
			}
			req, ok := syncMsg.Message.(*SyncMessage_Request)
			if !ok {
				errors <- fmt.Errorf("client %d expected SyncRequest but got %T", clientID, syncMsg.Message)
				return
			}
			if req.Request.ProcessorId == "" {
				errors <- fmt.Errorf("client %d received SyncRequest with empty ProcessorId", clientID)
				return
			}

			// Send ADD update
			expectedVersion := int64(clientID + 1)
			err = stream.Send(&CacheUpdate{
				Version:    expectedVersion,
				IsSnapshot: false,
				Operation:  OperationType_OPERATION_ADD,
				Entries:    []*ResourceEntry{{Kind: "Pod", Name: fmt.Sprintf("pod-%d", clientID), Namespace: "default"}},
			})
			if err != nil {
				errors <- fmt.Errorf("client %d failed to send ADD update: %w", clientID, err)
				return
			}

			// Receive ACK
			ackMsg, err := stream.Recv()
			if err != nil {
				errors <- fmt.Errorf("client %d failed to receive ACK: %w", clientID, err)
				return
			}
			ack, ok := ackMsg.Message.(*SyncMessage_Ack)
			if !ok {
				errors <- fmt.Errorf("client %d expected SyncAck but got %T", clientID, ackMsg.Message)
				return
			}
			if !ack.Ack.Success {
				errors <- fmt.Errorf("client %d received ACK with Success=false, error: %s", clientID, ack.Ack.Error)
				return
			}
			if ack.Ack.Version != expectedVersion {
				errors <- fmt.Errorf("client %d expected ACK version %d but got %d", clientID, expectedVersion, ack.Ack.Version)
				return
			}

			_ = stream.CloseSend()
			done <- true
		}(i)
	}

	// Wait for all clients
	for range numClients {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Fatal(err)
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for clients")
		}
	}

	fmt.Printf("✓ Integration test passed: %d clients connected successfully\n", numClients)
}
