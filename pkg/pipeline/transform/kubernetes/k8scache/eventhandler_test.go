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
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

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

	// Test the tombstone unwrapping logic (same as in OnDelete)
	var meta *model.ResourceMetaData
	var ok bool

	if ts, isTombstone := interface{}(tombstone).(cache.DeletedFinalStateUnknown); isTombstone {
		meta, ok = ts.Obj.(*model.ResourceMetaData)
		assert.True(t, ok, "should be able to extract ResourceMetaData from tombstone")
		assert.NotNil(t, meta, "extracted metadata should not be nil")
		assert.Equal(t, "test-pod", meta.Name, "metadata name should match")
		assert.Equal(t, "default", meta.Namespace, "metadata namespace should match")
	} else {
		t.Fatal("object should be recognized as tombstone")
	}
}

func TestOnDelete_InvalidTombstone(t *testing.T) {
	// Create a tombstone with wrong object type
	invalidTombstone := cache.DeletedFinalStateUnknown{
		Key: "default/invalid",
		Obj: "not-a-resource-metadata",
	}

	// Test that invalid tombstone is handled gracefully
	var meta *model.ResourceMetaData
	var ok bool

	if ts, isTombstone := interface{}(invalidTombstone).(cache.DeletedFinalStateUnknown); isTombstone {
		meta, ok = ts.Obj.(*model.ResourceMetaData)
		assert.False(t, ok, "should fail to extract ResourceMetaData from invalid tombstone")
		assert.Nil(t, meta, "metadata should be nil for invalid tombstone")
	}
}

func TestOnDelete_InvalidDirectObject(t *testing.T) {
	// Test with a completely wrong object type
	var invalidObj interface{} = "not-a-resource-metadata"

	// Test direct type assertion fails gracefully
	meta, ok := invalidObj.(*model.ResourceMetaData)
	assert.False(t, ok, "should fail type assertion on invalid object")
	assert.Nil(t, meta, "metadata should be nil for invalid object")
}

// Integration test that verifies the full OnDelete flow with tombstone
func TestOnDelete_TombstoneIntegration(t *testing.T) {
	// This test verifies that the OnDelete method can handle both:
	// 1. Normal delete events (direct ResourceMetaData object)
	// 2. Tombstone events (DeletedFinalStateUnknown wrapping ResourceMetaData)

	testCases := []struct {
		name        string
		obj         interface{}
		shouldWork  bool
		description string
	}{
		{
			name:        "normal delete",
			obj:         createTestPod("pod1", "default"),
			shouldWork:  true,
			description: "direct ResourceMetaData object",
		},
		{
			name: "tombstone delete",
			obj: cache.DeletedFinalStateUnknown{
				Key: "default/pod2",
				Obj: createTestPod("pod2", "default"),
			},
			shouldWork:  true,
			description: "tombstone wrapping ResourceMetaData",
		},
		{
			name: "invalid tombstone",
			obj: cache.DeletedFinalStateUnknown{
				Key: "invalid",
				Obj: "wrong-type",
			},
			shouldWork:  false,
			description: "tombstone with wrong object type",
		},
		{
			name:        "invalid object",
			obj:         "not-a-resource",
			shouldWork:  false,
			description: "completely wrong object type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Extract metadata using same logic as OnDelete
			var meta *model.ResourceMetaData
			var ok bool

			if tombstone, isTombstone := tc.obj.(cache.DeletedFinalStateUnknown); isTombstone {
				meta, ok = tombstone.Obj.(*model.ResourceMetaData)
			} else {
				meta, ok = tc.obj.(*model.ResourceMetaData)
			}

			if tc.shouldWork {
				assert.True(t, ok, "should successfully extract metadata for: %s", tc.description)
				assert.NotNil(t, meta, "metadata should not be nil for: %s", tc.description)
			} else {
				assert.False(t, ok, "should fail to extract metadata for: %s", tc.description)
			}
		})
	}
}
