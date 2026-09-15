package frr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

func TestInformerStore_StopCancelsInFlightList(t *testing.T) {
	origTimeout := syncTimeout
	syncTimeout = 200 * time.Millisecond
	t.Cleanup(func() { syncTimeout = origTimeout })

	s := NewInformerStore()

	listEntered := make(chan struct{}, 1)
	listExited := make(chan struct{}, 1)

	lw := &cache.ListWatch{
		ListFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
			select {
			case listEntered <- struct{}{}:
			default:
			}
			<-s.ctx.Done()
			select {
			case listExited <- struct{}{}:
			default:
			}
			return nil, s.ctx.Err()
		},
		WatchFunc: func(_ metav1.ListOptions) (watch.Interface, error) {
			<-s.ctx.Done()
			return nil, s.ctx.Err()
		},
	}

	go s.startInformer(lw) //nolint:errcheck

	<-listEntered
	s.Stop()

	select {
	case <-listExited:
	case <-time.After(2 * time.Second):
		t.Fatal("ListFunc did not exit after Stop cancelled its context")
	}
}

func TestInformerStore_NormalizesCIDRsOnMerge(t *testing.T) {
	s := NewInformerStore()
	err := s.LoadConfigs(
		&unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "a", "namespace": "ns"},
			"spec": map[string]interface{}{
				"bgp": map[string]interface{}{
					"routers": []interface{}{
						map[string]interface{}{
							"asn":      int64(64513),
							"prefixes": []interface{}{"10.128.1.0/14"},
						},
					},
				},
			},
		}},
		&unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "b", "namespace": "ns"},
			"spec": map[string]interface{}{
				"bgp": map[string]interface{}{
					"routers": []interface{}{
						map[string]interface{}{
							"asn":      int64(64512),
							"prefixes": []interface{}{"10.128.0.0/14"},
						},
					},
				},
			},
		}},
	)
	require.NoError(t, err)

	asn, ok := s.Lookup("10.129.0.1")
	require.True(t, ok)
	require.Equal(t, uint32(64512), asn)
}

func TestInformerStore_UpsertErrorCleansStale(t *testing.T) {
	s := NewInformerStore()

	validObj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "cfg", "namespace": "ns"},
		"spec": map[string]interface{}{
			"bgp": map[string]interface{}{
				"routers": []interface{}{
					map[string]interface{}{
						"asn":      int64(64512),
						"prefixes": []interface{}{"10.0.0.0/8"},
					},
				},
			},
		},
	}}
	s.upsert(validObj)
	asn, ok := s.Lookup("10.1.2.3")
	require.True(t, ok)
	require.Equal(t, uint32(64512), asn)

	invalidObj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "cfg", "namespace": "ns"},
		"spec": map[string]interface{}{
			"bgp": map[string]interface{}{
				"routers": "not-a-slice",
			},
		},
	}}
	s.upsert(invalidObj)

	_, ok = s.Lookup("10.1.2.3")
	require.False(t, ok)
}
