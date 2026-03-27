/*
 * Copyright (C) 2024 Red Hat, Inc.
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

package k8scache

import (
	"sync"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// mockClient captures calls to Send* methods for testing
// It implements the clientSender interface needed by EventHandler
type mockClient struct {
	mu          sync.Mutex
	deletedMeta []*model.ResourceMetaData
	addedMeta   []*model.ResourceMetaData
	updatedMeta []*model.ResourceMetaData
	sendError   error
}

func (m *mockClient) SendDelete(entries []*model.ResourceMetaData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedMeta = append(m.deletedMeta, entries...)
	return m.sendError
}

func (m *mockClient) SendAdd(entries []*model.ResourceMetaData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addedMeta = append(m.addedMeta, entries...)
	return m.sendError
}

func (m *mockClient) SendUpdate(entries []*model.ResourceMetaData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatedMeta = append(m.updatedMeta, entries...)
	return m.sendError
}

func (m *mockClient) getDeleted() []*model.ResourceMetaData {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deletedMeta
}

// createTestPod creates a ResourceMetaData for testing
func createTestPod(name, namespace string) *model.ResourceMetaData {
	return &model.ResourceMetaData{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Kind: model.KindPod,
	}
}

func TestOnDelete_TombstoneEvent(t *testing.T) {
	testPod := createTestPod("test-pod", "default")

	// Create a tombstone wrapping our test pod
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/test-pod",
		Obj: testPod,
	}

	// Create mock client and handler
	mock := &mockClient{}
	handler := &EventHandler{client: mock}

	// Call the real OnDelete method with tombstone
	handler.OnDelete(tombstone)

	// Verify the handler extracted and sent the correct metadata
	deleted := mock.getDeleted()
	require.Len(t, deleted, 1, "should have sent one delete")
	assert.Equal(t, "test-pod", deleted[0].Name, "metadata name should match")
	assert.Equal(t, "default", deleted[0].Namespace, "metadata namespace should match")
	assert.Equal(t, model.KindPod, deleted[0].Kind, "metadata kind should match")
}

func TestOnDelete_InvalidTombstone(t *testing.T) {
	// Create a tombstone with wrong object type
	invalidTombstone := cache.DeletedFinalStateUnknown{
		Key: "default/invalid",
		Obj: "not-a-resource-metadata",
	}

	// Create mock client and handler
	mock := &mockClient{}
	handler := &EventHandler{client: mock}

	// Call the real OnDelete method with invalid tombstone
	// Should log warning and not send anything
	handler.OnDelete(invalidTombstone)

	// Verify nothing was sent (handler gracefully handled invalid tombstone)
	deleted := mock.getDeleted()
	assert.Empty(t, deleted, "should not send delete for invalid tombstone")
}

func TestOnDelete_InvalidDirectObject(t *testing.T) {
	// Test with a completely wrong object type (not a tombstone, not ResourceMetaData)
	var invalidObj interface{} = "not-a-resource-metadata"

	// Create mock client and handler
	mock := &mockClient{}
	handler := &EventHandler{client: mock}

	// Call the real OnDelete method with invalid object
	// Should log warning and not send anything
	handler.OnDelete(invalidObj)

	// Verify nothing was sent (handler gracefully handled invalid object)
	deleted := mock.getDeleted()
	assert.Empty(t, deleted, "should not send delete for invalid object")
}

// Integration test that verifies the full OnDelete flow with tombstone
func TestOnDelete_TombstoneIntegration(t *testing.T) {
	// This test verifies that the OnDelete method can handle both:
	// 1. Normal delete events (direct ResourceMetaData object)
	// 2. Tombstone events (DeletedFinalStateUnknown wrapping ResourceMetaData)

	testCases := []struct {
		name         string
		obj          interface{}
		shouldSend   bool
		expectedName string
		expectedNS   string
	}{
		{
			name:         "normal delete",
			obj:          createTestPod("pod1", "default"),
			shouldSend:   true,
			expectedName: "pod1",
			expectedNS:   "default",
		},
		{
			name: "tombstone delete",
			obj: cache.DeletedFinalStateUnknown{
				Key: "default/pod2",
				Obj: createTestPod("pod2", "default"),
			},
			shouldSend:   true,
			expectedName: "pod2",
			expectedNS:   "default",
		},
		{
			name: "invalid tombstone",
			obj: cache.DeletedFinalStateUnknown{
				Key: "invalid",
				Obj: "wrong-type",
			},
			shouldSend: false,
		},
		{
			name:       "invalid object",
			obj:        "not-a-resource",
			shouldSend: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock client and handler for each test case
			mock := &mockClient{}
			handler := &EventHandler{client: mock}

			// Call the real OnDelete method
			handler.OnDelete(tc.obj)

			// Verify the result
			deleted := mock.getDeleted()
			if tc.shouldSend {
				require.Len(t, deleted, 1, "should have sent one delete")
				assert.Equal(t, tc.expectedName, deleted[0].Name, "name should match")
				assert.Equal(t, tc.expectedNS, deleted[0].Namespace, "namespace should match")
				assert.Equal(t, model.KindPod, deleted[0].Kind, "kind should match")
			} else {
				assert.Empty(t, deleted, "should not send delete for invalid input")
			}
		})
	}
}
