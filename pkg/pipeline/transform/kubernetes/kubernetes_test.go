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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"testing"
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
	pod := fakePod("podName", "podNamespace", "podHostIP")
	podInterface := interface{}(pod)
	return []interface{}{podInterface}, nil
}

func fakePod(name, ns, host string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Status: v1.PodStatus{
			HostIP: host,
		},
	}
}

func (informerMock *InformerMock) GetIndexer() cache.Indexer {
	informerMock.Mock.Called()
	return &IndexerMock{}
}

func TestKubeData_getInfo(t *testing.T) {
	// Test with no informer
	kubeData := KubeData{}
	info, err := kubeData.GetInfo("1.2.3.4")
	require.EqualError(t, err, "can't find ip")
	require.Nil(t, info)

	// Test with mock pod informer
	expectedInfo := &Info{
		Type:      "Pod",
		Name:      "podName",
		Namespace: "podNamespace",
		HostIP:    "podHostIP",
		Owner:     Owner{Name: "podName", Type: "Pod"},
	}
	kubeData = KubeData{ipInformers: map[string]cache.SharedIndexInformer{}}
	informerMock := &InformerMock{}
	informerMock.On("GetIndexer", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	kubeData.ipInformers["Pod"] = informerMock
	info, err = kubeData.GetInfo("1.2.3.4")
	require.NoError(t, err)
	require.Equal(t, info, expectedInfo)
}
