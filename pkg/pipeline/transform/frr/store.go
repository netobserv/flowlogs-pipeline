/*
 * Copyright (C) 2026 IBM, Inc.
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
 */

package frr

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/utils/k8sutils"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

const (
	frrGroup    = "frrk8s.metallb.io"
	frrVersion  = "v1beta1"
	frrResource = "frrconfigurations"
	syncPeriod  = 10 * time.Minute
)

var (
	log = logrus.WithField("component", "transform.Network.FRR")
	// Overridable in tests.
	syncTimeout      = 30 * time.Second
	probeListTimeout = 10 * time.Second
)

var frrGVR = schema.GroupVersionResource{
	Group:    frrGroup,
	Version:  frrVersion,
	Resource: frrResource,
}

// Store indexes FRRConfiguration advertised prefixes into an LPM ASN table.
type Store interface {
	Lookup(ipStr string) (uint32, bool)
}

// InformerStore watches FRRConfiguration resources and maintains an ASN LPM table.
type InformerStore struct {
	mu    sync.RWMutex
	raw   map[string]map[string]uint32 // object key → cidr→asn
	table atomic.Pointer[ASNTable]

	informer cache.SharedIndexInformer
	stopCh   chan struct{}
}

func NewInformerStore() *InformerStore {
	s := &InformerStore{
		raw:    make(map[string]map[string]uint32),
		stopCh: make(chan struct{}),
	}
	s.table.Store(BuildASNTable(nil))
	return s
}

func (s *InformerStore) Lookup(ipStr string) (uint32, bool) {
	return s.table.Load().LookupString(ipStr)
}

// Start watches FRRConfiguration resources cluster-wide.
func (s *InformerStore) Start(kubeConfigPath string) error {
	kconf, err := k8sutils.LoadK8sConfig(kubeConfigPath)
	if err != nil {
		return fmt.Errorf("loading kubeconfig for FRR informer: %w", err)
	}
	dynClient, err := dynamic.NewForConfig(kconf)
	if err != nil {
		return fmt.Errorf("creating dynamic client for FRR informer: %w", err)
	}

	probeCtx, probeCancel := context.WithTimeout(context.Background(), probeListTimeout)
	defer probeCancel()
	if _, err := dynClient.Resource(frrGVR).Namespace(metav1.NamespaceAll).List(probeCtx, metav1.ListOptions{Limit: 1}); err != nil {
		return fmt.Errorf("listing FRRConfigurations (is the frr-k8s CRD installed?): %w", err)
	}

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return dynClient.Resource(frrGVR).Namespace(metav1.NamespaceAll).List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return dynClient.Resource(frrGVR).Namespace(metav1.NamespaceAll).Watch(context.Background(), options)
		},
	}
	return s.startInformer(lw)
}

func (s *InformerStore) startInformer(lw cache.ListerWatcher) error {
	s.informer = cache.NewSharedIndexInformer(lw, &unstructured.Unstructured{}, syncPeriod, cache.Indexers{})
	_, err := s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			s.upsert(obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			s.upsert(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			s.remove(obj)
		},
	})
	if err != nil {
		return fmt.Errorf("adding FRR informer handlers: %w", err)
	}

	go s.informer.Run(s.stopCh)
	syncCtx, syncCancel := context.WithTimeout(context.Background(), syncTimeout)
	defer syncCancel()
	if !cache.WaitForCacheSync(syncCtx.Done(), s.informer.HasSynced) {
		s.Stop()
		return fmt.Errorf("timed out waiting for FRRConfiguration informer sync")
	}
	log.Infof("FRRConfiguration informer started (%d prefixes indexed)", s.table.Load().Len())
	return nil
}

func (s *InformerStore) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

func (s *InformerStore) upsert(obj interface{}) {
	u, ok := asUnstructured(obj)
	if !ok {
		return
	}
	mappings, err := extractASNMappings(u)
	if err != nil {
		log.WithError(err).Warnf("failed to extract FRRConfiguration %s/%s", u.GetNamespace(), u.GetName())
		return
	}
	key := objectKey(u)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.raw[key] = mappings
	s.rebuildLocked()
}

func (s *InformerStore) remove(obj interface{}) {
	u, ok := asUnstructured(obj)
	if !ok {
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			u, ok = asUnstructured(tombstone.Obj)
			if !ok {
				return
			}
		} else {
			return
		}
	}
	key := objectKey(u)

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.raw, key)
	s.rebuildLocked()
}

func (s *InformerStore) rebuildLocked() {
	merged := make(map[string]uint32)
	for _, mappings := range s.raw {
		for cidr, asn := range mappings {
			merged[cidr] = asn
		}
	}
	s.table.Store(BuildASNTable(merged))
}

// LoadConfigs populates the store without a live API server (tests).
func (s *InformerStore) LoadConfigs(configs ...*unstructured.Unstructured) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.raw = make(map[string]map[string]uint32)
	for _, cfg := range configs {
		mappings, err := extractASNMappings(cfg)
		if err != nil {
			return err
		}
		s.raw[objectKey(cfg)] = mappings
	}
	s.rebuildLocked()
	return nil
}

func asUnstructured(obj interface{}) (*unstructured.Unstructured, bool) {
	switch v := obj.(type) {
	case *unstructured.Unstructured:
		return v, true
	case unstructured.Unstructured:
		return &v, true
	default:
		return nil, false
	}
}

func objectKey(u *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s", u.GetNamespace(), u.GetName())
}
