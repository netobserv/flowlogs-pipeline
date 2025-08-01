/*
 * Copyright (C) 2021 IBM, Inc.
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

package informers

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/cni"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	"github.com/netobserv/flowlogs-pipeline/pkg/utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	inf "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
)

const (
	kubeConfigEnvVariable = "KUBECONFIG"
	syncTime              = 10 * time.Minute
	IndexCustom           = "byCustomKey"
	IndexIP               = "byIP"
)

var (
	log = logrus.WithField("component", "transform.Network.Kubernetes")
)

type Interface interface {
	IndexLookup([]cni.SecondaryNetKey, string) *model.ResourceMetaData
	GetNodeByName(string) (*model.ResourceMetaData, error)
	InitFromConfig(string, Config, *operational.Metrics) error
}

type Informers struct {
	Interface
	// pods, nodes and services cache the different object types as *Info pointers
	pods     cache.SharedIndexInformer
	nodes    cache.SharedIndexInformer
	services cache.SharedIndexInformer
	// replicaSets caches the ReplicaSets as partially-filled *ObjectMeta pointers
	replicaSets      cache.SharedIndexInformer
	stopChan         chan struct{}
	mdStopChan       chan struct{}
	indexerHitMetric *prometheus.CounterVec
}

var (
	ipIndexer = func(obj interface{}) ([]string, error) {
		return obj.(*model.ResourceMetaData).IPs, nil
	}
	customKeyIndexer = func(obj interface{}) ([]string, error) {
		return obj.(*model.ResourceMetaData).SecondaryNetKeys, nil
	}
)

func (k *Informers) IndexLookup(potentialKeys []cni.SecondaryNetKey, ip string) *model.ResourceMetaData {
	if info, ok := k.fetchInformers(potentialKeys, ip); ok {
		k.checkParent(info)
		return info
	}

	return nil
}

func (k *Informers) fetchInformers(potentialKeys []cni.SecondaryNetKey, ip string) (*model.ResourceMetaData, bool) {
	if info, ok := k.fetchPodInformer(potentialKeys, ip); ok {
		// it might happen that the Host is discovered after the Pod
		if info.HostName == "" {
			info.HostName = k.getHostName(info.HostIP)
		}
		return info, true
	}
	// Nodes are only indexed by IP
	if info, ok := k.infoForIP(k.nodes.GetIndexer(), "Node", ip); ok {
		return info, true
	}
	// Services are only indexed by IP
	if info, ok := k.infoForIP(k.services.GetIndexer(), "Service", ip); ok {
		return info, true
	}
	return nil, false
}

func (k *Informers) fetchPodInformer(potentialKeys []cni.SecondaryNetKey, ip string) (*model.ResourceMetaData, bool) {
	// 1. Check if the unique key matches any Pod (secondary networks / multus case)
	if info, ok := k.infoForCustomKeys(k.pods.GetIndexer(), "Pod", potentialKeys); ok {
		return info, ok
	}
	// 2. Check if the IP matches any Pod (primary network)
	return k.infoForIP(k.pods.GetIndexer(), "Pod", ip)
}

func (k *Informers) increaseIndexerHits(kind, namespace, network, warn string) {
	k.indexerHitMetric.WithLabelValues(kind, namespace, network, warn).Inc()
}

func (k *Informers) infoForCustomKeys(idx cache.Indexer, kind string, potentialKeys []cni.SecondaryNetKey) (*model.ResourceMetaData, bool) {
	for _, key := range potentialKeys {
		objs, err := idx.ByIndex(IndexCustom, key.Key)
		if err != nil {
			k.increaseIndexerHits(kind, "", key.NetworkName, "informer error")
			log.WithError(err).WithField("key", key).Debug("error accessing unique key index, ignoring")
			return nil, false
		}
		if len(objs) > 0 {
			info := objs[0].(*model.ResourceMetaData)
			info.NetworkName = key.NetworkName
			if len(objs) > 1 {
				k.increaseIndexerHits(kind, info.Namespace, key.NetworkName, "multiple matches")
				log.WithField("key", key).Debugf("found %d objects matching this key, returning first", len(objs))
			} else {
				k.increaseIndexerHits(kind, info.Namespace, key.NetworkName, "")
			}
			log.Tracef("infoForUniqueKey found key %v", info)
			return info, true
		}
	}
	return nil, false
}

func (k *Informers) infoForIP(idx cache.Indexer, kind string, ip string) (*model.ResourceMetaData, bool) {
	objs, err := idx.ByIndex(IndexIP, ip)
	if err != nil {
		k.increaseIndexerHits(kind, "", "primary", "informer error")
		log.WithError(err).WithField("ip", ip).Debug("error accessing IP index, ignoring")
		return nil, false
	}
	if len(objs) > 0 {
		info := objs[0].(*model.ResourceMetaData)
		info.NetworkName = "primary"
		if len(objs) > 1 {
			k.increaseIndexerHits(kind, info.Namespace, "primary", "multiple matches")
			log.WithField("ip", ip).Debugf("found %d objects matching this IP, returning first", len(objs))
		} else {
			k.increaseIndexerHits(kind, info.Namespace, "primary", "")
		}
		log.Tracef("infoForIP found ip %v", info)
		return info, true
	}
	return nil, false
}

func (k *Informers) GetNodeByName(name string) (*model.ResourceMetaData, error) {
	item, ok, err := k.nodes.GetIndexer().GetByKey(name)
	if err != nil {
		return nil, err
	} else if ok {
		return item.(*model.ResourceMetaData), nil
	}
	return nil, nil
}

func (k *Informers) checkParent(info *model.ResourceMetaData) {
	if info.OwnerKind == "ReplicaSet" {
		item, ok, err := k.replicaSets.GetIndexer().GetByKey(info.Namespace + "/" + info.OwnerName)
		if err != nil {
			log.WithError(err).WithField("key", info.Namespace+"/"+info.OwnerName).
				Debug("can't get ReplicaSet info from informer. Ignoring")
		} else if ok {
			rsInfo := item.(*metav1.ObjectMeta)
			if len(rsInfo.OwnerReferences) > 0 {
				info.OwnerKind = rsInfo.OwnerReferences[0].Kind
				info.OwnerName = rsInfo.OwnerReferences[0].Name
			}
		}
	}
}

func (k *Informers) getHostName(hostIP string) string {
	if hostIP != "" {
		if info, ok := k.infoForIP(k.nodes.GetIndexer(), "Node (indirect)", hostIP); ok {
			return info.Name
		}
	}
	return ""
}

func (k *Informers) initNodeInformer(informerFactory inf.SharedInformerFactory, cfg Config) error {
	nodes := informerFactory.Core().V1().Nodes().Informer()
	// Transform any *v1.Node instance into a *Info instance to save space
	// in the informer's cache
	if err := nodes.SetTransform(func(i interface{}) (interface{}, error) {
		node, ok := i.(*v1.Node)
		if !ok {
			return nil, fmt.Errorf("was expecting a Node. Got: %T", i)
		}
		ips := make([]string, 0, len(node.Status.Addresses))
		hostIP := ""
		for _, address := range node.Status.Addresses {
			ip := net.ParseIP(address.Address)
			if ip != nil {
				ips = append(ips, ip.String())
				if hostIP == "" {
					hostIP = ip.String()
				}
			}
		}
		// CNI-dependent logic (must not fail when the CNI is not installed)
		for _, name := range cfg.managedCNI {
			if plugin := cniPlugins[name]; plugin != nil {
				moreIPs := plugin.GetNodeIPs(node)
				if moreIPs != nil {
					ips = append(ips, moreIPs...)
				}
			}
		}

		return &model.ResourceMetaData{
			ObjectMeta: metav1.ObjectMeta{
				Name:   node.Name,
				Labels: node.Labels,
			},
			Kind:      model.KindNode,
			OwnerName: node.Name,
			OwnerKind: model.KindNode,
			IPs:       ips,
			// We duplicate HostIP and HostName information to simplify later filtering e.g. by
			// Host IP, where we want to get all the Pod flows by src/dst host, but also the actual
			// host-to-host flows by the same field.
			HostIP:   hostIP,
			HostName: node.Name,
		}, nil
	}); err != nil {
		return fmt.Errorf("can't set nodes transform: %w", err)
	}
	k.nodes = nodes
	return nil
}

func (k *Informers) initPodInformer(informerFactory inf.SharedInformerFactory, cfg Config, dynClient *dynamic.DynamicClient) error {
	pods := informerFactory.Core().V1().Pods().Informer()
	// Transform any *v1.Pod instance into a *Info instance to save space
	// in the informer's cache
	if err := pods.SetTransform(func(i interface{}) (interface{}, error) {
		pod, ok := i.(*v1.Pod)
		if !ok {
			return nil, fmt.Errorf("was expecting a Pod. Got: %T", i)
		}
		ips := make([]string, 0, len(pod.Status.PodIPs))
		for _, ip := range pod.Status.PodIPs {
			// ignoring host-networked Pod IPs
			if ip.IP != pod.Status.HostIP {
				ips = append(ips, ip.IP)
			}
		}
		// Index from secondary network info
		var keys []string
		var err error
		if cfg.hasMultus {
			keys, err = multus.GetPodUniqueKeys(pod, cfg.secondaryNetworks)
			if err != nil {
				// Log the error as Info, do not block other ips indexing
				log.WithError(err).Infof("Secondary network cannot be identified")
			}
		}
		if cfg.hasUDN {
			if udnKeys, err := udn.GetPodUniqueKeys(context.Background(), dynClient, pod); err != nil {
				// Log the error as Info, do not block other ips indexing
				log.WithError(err).Infof("UDNs cannot be identified")
			} else {
				keys = append(keys, udnKeys...)
			}
		}

		obj := model.ResourceMetaData{
			ObjectMeta: metav1.ObjectMeta{
				Name:            pod.Name,
				Namespace:       pod.Namespace,
				Labels:          pod.Labels,
				OwnerReferences: pod.OwnerReferences,
			},
			Kind:             model.KindPod,
			OwnerName:        pod.Name,
			OwnerKind:        model.KindPod,
			HostIP:           pod.Status.HostIP,
			HostName:         pod.Spec.NodeName,
			SecondaryNetKeys: keys,
			IPs:              ips,
		}
		if len(pod.OwnerReferences) > 0 {
			obj.OwnerKind = pod.OwnerReferences[0].Kind
			obj.OwnerName = pod.OwnerReferences[0].Name
		}
		k.checkParent(&obj)
		return &obj, nil
	}); err != nil {
		return fmt.Errorf("can't set pods transform: %w", err)
	}
	k.pods = pods
	return nil
}

func (k *Informers) initServiceInformer(informerFactory inf.SharedInformerFactory) error {
	services := informerFactory.Core().V1().Services().Informer()
	// Transform any *v1.Service instance into a *Info instance to save space
	// in the informer's cache
	if err := services.SetTransform(func(i interface{}) (interface{}, error) {
		svc, ok := i.(*v1.Service)
		if !ok {
			return nil, fmt.Errorf("was expecting a Service. Got: %T", i)
		}
		ips := make([]string, 0, len(svc.Spec.ClusterIPs))
		for _, ip := range svc.Spec.ClusterIPs {
			// ignoring None IPs
			if isServiceIPSet(ip) {
				ips = append(ips, ip)
			}
		}
		return &model.ResourceMetaData{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svc.Name,
				Namespace: svc.Namespace,
				Labels:    svc.Labels,
			},
			Kind:      model.KindService,
			OwnerName: svc.Name,
			OwnerKind: model.KindService,
			IPs:       ips,
		}, nil
	}); err != nil {
		return fmt.Errorf("can't set services transform: %w", err)
	}
	k.services = services
	return nil
}

func (k *Informers) initReplicaSetInformer(informerFactory metadatainformer.SharedInformerFactory) error {
	k.replicaSets = informerFactory.ForResource(
		schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "replicasets",
		}).Informer()
	// To save space, instead of storing a complete *metav1.ObjectMeta instance, the
	// informer's cache will store only the minimal required fields
	if err := k.replicaSets.SetTransform(func(i interface{}) (interface{}, error) {
		rs, ok := i.(*metav1.PartialObjectMetadata)
		if !ok {
			return nil, fmt.Errorf("was expecting a ReplicaSet. Got: %T", i)
		}
		return &metav1.ObjectMeta{
			Name:            rs.Name,
			Namespace:       rs.Namespace,
			OwnerReferences: rs.OwnerReferences,
		}, nil
	}); err != nil {
		return fmt.Errorf("can't set ReplicaSets transform: %w", err)
	}
	return nil
}

func (k *Informers) InitFromConfig(kubeconfig string, infConfig Config, opMetrics *operational.Metrics) error {
	// Initialization variables
	k.stopChan = make(chan struct{})
	k.mdStopChan = make(chan struct{})

	kconf, err := utils.LoadK8sConfig(kubeconfig)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(kconf)
	if err != nil {
		return err
	}

	metaKubeClient, err := metadata.NewForConfig(kconf)
	if err != nil {
		return err
	}

	dynClient, err := dynamic.NewForConfig(kconf)
	if err != nil {
		return err
	}

	k.indexerHitMetric = opMetrics.CreateIndexerHitCounter()
	err = k.initInformers(kubeClient, metaKubeClient, dynClient, infConfig)
	if err != nil {
		return err
	}

	return nil
}

func (k *Informers) initInformers(client kubernetes.Interface, metaClient metadata.Interface, dynClient *dynamic.DynamicClient, cfg Config) error {
	informerFactory := inf.NewSharedInformerFactory(client, syncTime)
	metadataInformerFactory := metadatainformer.NewSharedInformerFactory(metaClient, syncTime)
	err := k.initNodeInformer(informerFactory, cfg)
	if err != nil {
		return err
	}
	err = k.initPodInformer(informerFactory, cfg, dynClient)
	if err != nil {
		return err
	}
	err = k.initServiceInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.initReplicaSetInformer(metadataInformerFactory)
	if err != nil {
		return err
	}

	// Informers expose an indexer
	log.Debugf("adding indexers")
	byIP := cache.Indexers{IndexIP: ipIndexer}
	byIPAndCustom := cache.Indexers{
		IndexIP:     ipIndexer,
		IndexCustom: customKeyIndexer,
	}
	if err := k.nodes.AddIndexers(byIP); err != nil {
		return fmt.Errorf("can't add indexers to Nodes informer: %w", err)
	}
	if err := k.pods.AddIndexers(byIPAndCustom); err != nil {
		return fmt.Errorf("can't add indexers to Pods informer: %w", err)
	}
	if err := k.services.AddIndexers(byIP); err != nil {
		return fmt.Errorf("can't add indexers to Services informer: %w", err)
	}

	log.Debugf("starting kubernetes informers, waiting for synchronization")
	informerFactory.Start(k.stopChan)
	informerFactory.WaitForCacheSync(k.stopChan)
	log.Debugf("kubernetes informers started")

	log.Debugf("starting kubernetes metadata informers, waiting for synchronization")
	metadataInformerFactory.Start(k.mdStopChan)
	metadataInformerFactory.WaitForCacheSync(k.mdStopChan)
	log.Debugf("kubernetes metadata informers started")
	return nil
}

func isServiceIPSet(ip string) bool {
	return ip != v1.ClusterIPNone && ip != ""
}
