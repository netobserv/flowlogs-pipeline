package informers

import (
	"errors"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type Mock struct {
	mock.Mock
	InformersInterface
}

func NewInformersMock() *Mock {
	inf := new(Mock)
	inf.On("InitFromConfig", mock.Anything).Return(nil)
	return inf
}

func (o *Mock) InitFromConfig(cfg api.NetworkTransformKubeConfig) error {
	args := o.Called(cfg)
	return args.Error(0)
}

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

func (m *IndexerMock) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	args := m.Called(indexName, indexedValue)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *IndexerMock) GetByKey(key string) (interface{}, bool, error) {
	args := m.Called(key)
	return args.Get(0), args.Bool(1), args.Error(2)
}

func (m *InformerMock) GetIndexer() cache.Indexer {
	args := m.Called()
	return args.Get(0).(cache.Indexer)
}

func (m *IndexerMock) MockPod(ip, mac, name, namespace, nodeIP string, owner *Owner) {
	var ownerRef []metav1.OwnerReference
	if owner != nil {
		ownerRef = []metav1.OwnerReference{{
			Kind: owner.Type,
			Name: owner.Name,
		}}
	}
	info := Info{
		Type: "Pod",
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: ownerRef,
		},
		HostIP: nodeIP,
		ips:    []string{},
		macs:   []string{},
	}
	if len(mac) > 0 {
		info.macs = []string{mac}
		m.On("ByIndex", IndexMAC, mac).Return([]interface{}{&info}, nil)
	}
	if len(ip) > 0 {
		info.ips = []string{ip}
		m.On("ByIndex", IndexIP, ip).Return([]interface{}{&info}, nil)
	}
}

func (m *IndexerMock) MockNode(ip, name string) {
	m.On("ByIndex", IndexIP, ip).Return([]interface{}{&Info{
		Type:       "Node",
		ObjectMeta: metav1.ObjectMeta{Name: name},
		ips:        []string{ip},
		macs:       []string{},
	}}, nil)
}

func (m *IndexerMock) MockService(ip, name, namespace string) {
	m.On("ByIndex", IndexIP, ip).Return([]interface{}{&Info{
		Type:       "Service",
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		ips:        []string{ip},
		macs:       []string{},
	}}, nil)
}

func (m *IndexerMock) MockReplicaSet(name, namespace string, owner Owner) {
	m.On("GetByKey", namespace+"/"+name).Return(&metav1.ObjectMeta{
		Name: name,
		OwnerReferences: []metav1.OwnerReference{{
			Kind: owner.Type,
			Name: owner.Name,
		}},
	}, true, nil)
}

func (m *IndexerMock) FallbackNotFound() {
	m.On("ByIndex", IndexIP, mock.Anything).Return([]interface{}{}, nil)
}

func SetupIndexerMocks(kd *Informers) (pods, nodes, svc, rs *IndexerMock) {
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

type FakeInformers struct {
	InformersInterface
	ipInfo  map[string]*Info
	macInfo map[string]*Info
	nodes   map[string]*Info
}

func SetupStubs(ipInfo map[string]*Info, macInfo map[string]*Info, nodes map[string]*Info) *FakeInformers {
	return &FakeInformers{
		ipInfo:  ipInfo,
		macInfo: macInfo,
		nodes:   nodes,
	}
}

func (f *FakeInformers) InitFromConfig(_ api.NetworkTransformKubeConfig) error {
	return nil
}

func (f *FakeInformers) GetInfo(ip string, mac string) (*Info, error) {
	if len(mac) > 0 {
		i := f.macInfo[mac]
		if i != nil {
			return i, nil
		}
	}

	i := f.ipInfo[ip]
	if i != nil {
		return i, nil
	}
	return nil, errors.New("notFound")
}

func (f *FakeInformers) GetNodeInfo(n string) (*Info, error) {
	i := f.nodes[n]
	if i != nil {
		return i, nil
	}
	return nil, errors.New("notFound")
}
