package datasource

import (
	"sync/atomic"

	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/informers"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
)

type Datasource struct {
	Informers informers.Interface
	// kubernetesStore, when set, is used for IndexLookup and GetNodeByName instead of Informers.
	// It is populated by the k8s cache sync gRPC server when receiving updates from flp-informers.
	// Access is synchronized via atomic operations to prevent race conditions.
	kubernetesStore atomic.Pointer[KubernetesStore]
}

// SetKubernetesStore sets the Kubernetes store (used when k8s cache server is enabled).
// This method is thread-safe and can be called concurrently with lookups.
func (d *Datasource) SetKubernetesStore(store *KubernetesStore) {
	d.kubernetesStore.Store(store)
}

func NewInformerDatasource(kubeconfig string, infConfig *informers.Config, opMetrics *operational.Metrics) (*Datasource, error) {
	inf := &informers.Informers{}
	if err := inf.InitFromConfig(kubeconfig, infConfig, opMetrics); err != nil {
		return nil, err
	}
	return &Datasource{Informers: inf}, nil
}

func (d *Datasource) IndexLookup(potentialKeys []string, ip string) *model.ResourceMetaData {
	if store := d.kubernetesStore.Load(); store != nil {
		return store.IndexLookup(potentialKeys, ip)
	}
	return d.Informers.IndexLookup(potentialKeys, ip)
}

func (d *Datasource) GetNodeByName(name string) (*model.ResourceMetaData, error) {
	if store := d.kubernetesStore.Load(); store != nil {
		return store.GetNodeByName(name)
	}
	return d.Informers.GetNodeByName(name)
}

// ApplyCacheAddOrUpdate adds or updates the given entries in the Kubernetes store.
// This method is thread-safe and can be called concurrently.
func (d *Datasource) ApplyCacheAddOrUpdate(entries []*model.ResourceMetaData) {
	if store := d.kubernetesStore.Load(); store != nil {
		store.AddOrUpdate(entries)
	}
}

// ApplyCacheDelete removes the given entries from the Kubernetes store.
// This method is thread-safe and can be called concurrently.
func (d *Datasource) ApplyCacheDelete(entries []*model.ResourceMetaData) {
	if store := d.kubernetesStore.Load(); store != nil {
		store.Delete(entries)
	}
}
