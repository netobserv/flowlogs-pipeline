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

package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type IndexerMock struct {
	mock.Mock
	cache.Indexer
}

type InformerMock struct {
	mock.Mock
	InformerInterface
}

type InformerInterface interface {
	cache.SharedInformer
	AddIndexers(indexers cache.Indexers) error
	GetIndexer() cache.Indexer
}

func (indexMock *IndexerMock) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	args := indexMock.Called(indexName, indexedValue)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (indexMock *IndexerMock) GetByKey(key string) (interface{}, bool, error) {
	args := indexMock.Called(key)
	return args.Get(0), args.Bool(1), args.Error(2)
}

func (informerMock *InformerMock) GetIndexer() cache.Indexer {
	args := informerMock.Called()
	return args.Get(0).(cache.Indexer)
}

func (m *IndexerMock) mockPod(ip, name, namespace, nodeIP string, owner *Owner) {
	var ownerRef []metav1.OwnerReference
	if owner != nil {
		ownerRef = []metav1.OwnerReference{{
			Kind: owner.Type,
			Name: owner.Name,
		}}
	}
	m.On("ByIndex", IndexIP, ip).Return([]interface{}{&Info{
		Type: "Pod",
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: ownerRef,
		},
		HostIP: nodeIP,
	}}, nil)
}

func (m *IndexerMock) mockNode(ip, name string) {
	m.On("ByIndex", IndexIP, ip).Return([]interface{}{&Info{
		Type:       "Node",
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}}, nil)
}

func (m *IndexerMock) mockService(ip, name, namespace string) {
	m.On("ByIndex", IndexIP, ip).Return([]interface{}{&Info{
		Type:       "Service",
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}}, nil)
}

func (m *IndexerMock) mockReplicaSet(name, namespace string, owner Owner) {
	m.On("GetByKey", namespace+"/"+name).Return(&metav1.ObjectMeta{
		Name: name,
		OwnerReferences: []metav1.OwnerReference{{
			Kind: owner.Type,
			Name: owner.Name,
		}},
	}, true, nil)
}

func (m *IndexerMock) fallbackNotFound() {
	m.On("ByIndex", IndexIP, mock.Anything).Return([]interface{}{}, nil)
}

func setupMocks(kd *KubeData) (pods, nodes, svc, rs *IndexerMock) {
	// pods informer
	pods = &IndexerMock{}
	pim := InformerMock{}
	pim.On("GetIndexer").Return(pods)
	kd.pods = &pim
	// nodes informer
	nodes = &IndexerMock{}
	him := InformerMock{}
	him.On("GetIndexer").Return(nodes)
	kd.nodes = &him
	// svc informer
	svc = &IndexerMock{}
	sim := InformerMock{}
	sim.On("GetIndexer").Return(svc)
	kd.services = &sim
	// rs informer
	rs = &IndexerMock{}
	rim := InformerMock{}
	rim.On("GetIndexer").Return(rs)
	kd.replicaSets = &rim
	return
}

func TestGetInfo(t *testing.T) {
	kubeData := KubeData{}
	pidx, hidx, sidx, ridx := setupMocks(&kubeData)
	pidx.mockPod("1.2.3.4", "pod1", "podNamespace", "10.0.0.1", nil)
	pidx.mockPod("1.2.3.5", "pod2", "podNamespace", "10.0.0.1", &Owner{Name: "rs1", Type: "ReplicaSet"})
	pidx.fallbackNotFound()
	ridx.mockReplicaSet("rs1", "podNamespace", Owner{Name: "dep1", Type: "Deployment"})
	ridx.fallbackNotFound()
	sidx.mockService("1.2.3.100", "svc1", "svcNamespace")
	sidx.fallbackNotFound()
	hidx.mockNode("10.0.0.1", "node1")
	hidx.fallbackNotFound()

	// Test get orphan pod
	info, err := kubeData.GetInfo("1.2.3.4")
	require.NoError(t, err)

	require.Equal(t, *info, Info{
		Type: "Pod",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "podNamespace",
		},
		HostName: "node1",
		HostIP:   "10.0.0.1",
		Owner:    Owner{Name: "pod1", Type: "Pod"},
	})

	// Test get pod owned
	info, err = kubeData.GetInfo("1.2.3.5")
	require.NoError(t, err)

	require.Equal(t, *info, Info{
		Type: "Pod",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod2",
			Namespace: "podNamespace",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet",
				Name: "rs1",
			}},
		},
		HostName: "node1",
		HostIP:   "10.0.0.1",
		Owner:    Owner{Name: "dep1", Type: "Deployment"},
	})

	// Test get node
	info, err = kubeData.GetInfo("10.0.0.1")
	require.NoError(t, err)

	require.Equal(t, *info, Info{
		Type: "Node",
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
		Owner: Owner{Name: "node1", Type: "Node"},
	})

	// Test get service
	info, err = kubeData.GetInfo("1.2.3.100")
	require.NoError(t, err)

	require.Equal(t, *info, Info{
		Type: "Service",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc1",
			Namespace: "svcNamespace",
		},
		Owner: Owner{Name: "svc1", Type: "Service"},
	})

	// Test no match
	info, err = kubeData.GetInfo("1.2.3.200")
	require.NotNil(t, err)
	require.Nil(t, info)
}
