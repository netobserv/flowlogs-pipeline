package datasource

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestKubernetesStore_NilEntries verifies that nil entries are safely ignored
// without causing panics in Replace, AddOrUpdate, and Delete operations
func TestKubernetesStore_NilEntries(t *testing.T) {
	store := NewKubernetesStore()

	validEntry := &model.ResourceMetaData{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Kind: "Pod",
		IPs:  []string{"10.0.0.1"},
	}

	// Test Replace with nil entries
	t.Run("Replace with nil entries", func(t *testing.T) {
		entries := []*model.ResourceMetaData{
			validEntry,
			nil, // nil entry should be skipped
			validEntry,
		}

		// Should not panic
		require.NotPanics(t, func() {
			store.Replace(entries)
		})

		// Valid entry should be in the store
		result := store.IndexLookup(nil, "10.0.0.1")
		require.NotNil(t, result)
		require.Equal(t, "test-pod", result.Name)
	})

	// Test AddOrUpdate with nil entries
	t.Run("AddOrUpdate with nil entries", func(t *testing.T) {
		store = NewKubernetesStore()
		entries := []*model.ResourceMetaData{
			nil, // nil entry should be skipped
			validEntry,
			nil,
		}

		// Should not panic
		require.NotPanics(t, func() {
			store.AddOrUpdate(entries)
		})

		// Valid entry should be in the store
		result := store.IndexLookup(nil, "10.0.0.1")
		require.NotNil(t, result)
		require.Equal(t, "test-pod", result.Name)
	})

	// Test Delete with nil entries
	t.Run("Delete with nil entries", func(t *testing.T) {
		store = NewKubernetesStore()
		store.AddOrUpdate([]*model.ResourceMetaData{validEntry})

		entries := []*model.ResourceMetaData{
			nil, // nil entry should be skipped
			validEntry,
		}

		// Should not panic
		require.NotPanics(t, func() {
			store.Delete(entries)
		})

		// Entry should be deleted
		result := store.IndexLookup(nil, "10.0.0.1")
		require.Nil(t, result)
	})
}

// TestKubernetesStore_FirstMatchWins verifies that byIP honors "first match wins"
// and deletion only removes entries owned by the resource being deleted
func TestKubernetesStore_FirstMatchWins(t *testing.T) {
	store := NewKubernetesStore()

	// Two pods sharing the same IP (e.g., host network pods)
	pod1 := &model.ResourceMetaData{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns1",
			UID:       "uid-pod1",
		},
		Kind: "Pod",
		IPs:  []string{"10.0.0.100"},
	}

	pod2 := &model.ResourceMetaData{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod2",
			Namespace: "ns2",
			UID:       "uid-pod2",
		},
		Kind: "Pod",
		IPs:  []string{"10.0.0.100"}, // Same IP as pod1
	}

	// Add pod1 first
	store.AddOrUpdate([]*model.ResourceMetaData{pod1})

	// Verify pod1 is indexed by IP
	result := store.IndexLookup(nil, "10.0.0.100")
	require.NotNil(t, result)
	require.Equal(t, "pod1", result.Name)
	require.Equal(t, "uid-pod1", string(result.UID))

	// Add pod2 with same IP - should NOT overwrite (first match wins)
	store.AddOrUpdate([]*model.ResourceMetaData{pod2})

	// Verify pod1 is still the owner (first match wins)
	result = store.IndexLookup(nil, "10.0.0.100")
	require.NotNil(t, result)
	require.Equal(t, "pod1", result.Name, "First match should win - pod1 should still be indexed")
	require.Equal(t, "uid-pod1", string(result.UID))

	// Delete pod2 - should NOT remove the IP entry (pod1 owns it)
	store.Delete([]*model.ResourceMetaData{pod2})

	// Verify pod1 is still indexed (pod2 deletion didn't affect it)
	result = store.IndexLookup(nil, "10.0.0.100")
	require.NotNil(t, result, "IP should still be indexed by pod1")
	require.Equal(t, "pod1", result.Name)

	// Delete pod1 - should remove the IP entry (pod1 is the actual owner)
	store.Delete([]*model.ResourceMetaData{pod1})

	// Verify IP is now removed
	result = store.IndexLookup(nil, "10.0.0.100")
	require.Nil(t, result, "IP should be removed after deleting the owner")
}
