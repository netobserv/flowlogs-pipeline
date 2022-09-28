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

package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/cni"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var Data kubeDataInterface = &KubeData{}

const (
	kubeConfigEnvVariable = "KUBECONFIG"
	syncTime              = 10 * time.Minute
	IndexIP               = "byIP"
	typeNode              = "Node"
	typePod               = "Pod"
	typeService           = "Service"
)

type kubeDataInterface interface {
	GetInfo(string) (*Info, error)
	InitFromConfig(string) error
}

type KubeData struct {
	kubeDataInterface
	ipInformers        map[string]cache.SharedIndexInformer
	replicaSetInformer cache.SharedIndexInformer
	stopChan           chan struct{}
}

type Owner struct {
	Type string
	Name string
}

// Info contains precollected resources' metadata.
// Not all the Info types have all the data populated, as we aim to save
// memory and just keep in memory the necessary data.
// For more information about which data holds each type, please refer to the
// respective informers istantiation functions
type Info struct {
	// Informer's need that internal object have an ObjectMeta function
	metav1.ObjectMeta
	Type     string
	Owner    Owner
	HostName string
	HostIP   string
	ips      []string
}

var commonIndexers = map[string]cache.IndexFunc{
	IndexIP: func(obj interface{}) ([]string, error) {
		return obj.(*Info).ips, nil
	},
}

func (k *KubeData) GetInfo(ip string) (*Info, error) {
	for objType, informer := range k.ipInformers {
		objs, err := informer.GetIndexer().ByIndex(IndexIP, ip)
		if err == nil && len(objs) > 0 {
			var info *Info
			switch objType {
			case typePod:
				info = objs[0].(*Info)
				// it might happen that the Host is discovered after the Pod
				if info.HostName == "" {
					info.HostName = k.getHostName(info.HostIP)
				}
			case typeNode, typeService:
				info = objs[0].(*Info)
			}
			if info.Owner.Name == "" {
				info.Owner = k.getOwner(info)
			}
			return info, nil
		}
	}

	return nil, fmt.Errorf("can't find ip")
}

func (k *KubeData) getOwner(info *Info) Owner {
	if info.OwnerReferences != nil && len(info.OwnerReferences) > 0 {
		ownerReference := info.OwnerReferences[0]
		if ownerReference.Kind == "ReplicaSet" {
			item, ok, err := k.replicaSetInformer.GetIndexer().GetByKey(info.Namespace + "/" + ownerReference.Name)
			if err != nil {
				panic(err)
			}
			if ok {
				replicaSet := item.(*appsv1.ReplicaSet)
				if len(replicaSet.OwnerReferences) > 0 {
					return Owner{
						Name: replicaSet.OwnerReferences[0].Name,
						Type: replicaSet.OwnerReferences[0].Kind,
					}
				}
			}
		} else {
			return Owner{
				Name: ownerReference.Name,
				Type: ownerReference.Kind,
			}
		}
	}

	return Owner{
		Name: info.Name,
		Type: info.Type,
	}
}

func (k *KubeData) getHostName(hostIP string) string {
	if k.ipInformers[typeNode] != nil && len(hostIP) > 0 {
		objs, err := k.ipInformers[typeNode].GetIndexer().ByIndex(IndexIP, hostIP)
		if err == nil && len(objs) > 0 {
			return objs[0].(*Info).Name
		}
	}
	return ""
}

func (k *KubeData) initNodeInformer(informerFactory informers.SharedInformerFactory) error {
	nodes := informerFactory.Core().V1().Nodes().Informer()
	if err := nodes.SetTransform(func(i interface{}) (interface{}, error) {
		node, ok := i.(*v1.Node)
		if !ok {
			return nil, fmt.Errorf("was expecting a Node. Got: %T", i)
		}
		ips := make([]string, 0, len(node.Status.Addresses))
		for _, address := range node.Status.Addresses {
			ip := net.ParseIP(address.Address)
			if ip != nil {
				ips = append(ips, ip.String())
			}
		}
		// CNI-dependent logic (must work regardless of whether the CNI is installed)
		ips = cni.AddOvnIPs(ips, node)

		return &Info{
			ObjectMeta: metav1.ObjectMeta{
				Name:      node.Name,
				Namespace: node.Namespace,
				Labels:    node.Labels,
			},
			ips:  ips,
			Type: typeNode,
		}, nil
	}); err != nil {
		return fmt.Errorf("can't set pods transform: %w", err)
	}
	if err := nodes.AddIndexers(commonIndexers); err != nil {
		return fmt.Errorf("can't add %s indexer to Nodes informer: %w", IndexIP, err)
	}
	k.ipInformers[typeNode] = nodes
	return nil
}

func (k *KubeData) initPodInformer(informerFactory informers.SharedInformerFactory) error {
	pods := informerFactory.Core().V1().Pods().Informer()
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
		return &Info{
			ObjectMeta: metav1.ObjectMeta{
				Name:            pod.Name,
				Namespace:       pod.Namespace,
				Labels:          pod.Labels,
				OwnerReferences: pod.OwnerReferences,
			},
			Type:   typePod,
			HostIP: pod.Status.HostIP,
			ips:    ips,
		}, nil
	}); err != nil {
		return fmt.Errorf("can't set pods transform: %w", err)
	}
	if err := pods.AddIndexers(commonIndexers); err != nil {
		return fmt.Errorf("can't add %s indexer to Pods informer: %w", IndexIP, err)
	}

	k.ipInformers[typePod] = pods
	return nil
}

func (k *KubeData) initServiceInformer(informerFactory informers.SharedInformerFactory) error {
	services := informerFactory.Core().V1().Services().Informer()
	if err := services.SetTransform(func(i interface{}) (interface{}, error) {
		svc, ok := i.(*v1.Service)
		if !ok {
			return nil, fmt.Errorf("was expecting a Pod. Got: %T", i)
		}
		if svc.Spec.ClusterIP == v1.ClusterIPNone {
			return nil, errors.New("not indexing service without ClusterIP")
		}
		return &Info{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svc.Name,
				Namespace: svc.Namespace,
				Labels:    svc.Labels,
			},
			Type: typeService,
			ips:  svc.Spec.ClusterIPs,
		}, nil
	}); err != nil {
		return fmt.Errorf("can't set pods transform: %w", err)
	}
	if err := services.AddIndexers(commonIndexers); err != nil {
		return fmt.Errorf("can't add %s indexer to Pods informer: %w", IndexIP, err)
	}

	k.ipInformers[typeService] = services
	return nil
}

func (k *KubeData) initReplicaSetInformer(informerFactory informers.SharedInformerFactory) error {
	k.replicaSetInformer = informerFactory.Apps().V1().ReplicaSets().Informer()
	return nil
}

func (k *KubeData) InitFromConfig(kubeConfigPath string) error {
	// Initialization variables
	k.stopChan = make(chan struct{})
	k.ipInformers = map[string]cache.SharedIndexInformer{}

	config, err := LoadConfig(kubeConfigPath)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	err = k.initInformers(kubeClient)
	if err != nil {
		return err
	}

	return nil
}

func LoadConfig(kubeConfigPath string) (*rest.Config, error) {
	// if no config path is provided, load it from the env variable
	if kubeConfigPath == "" {
		kubeConfigPath = os.Getenv(kubeConfigEnvVariable)
	}
	// otherwise, load it from the $HOME/.kube/config file
	if kubeConfigPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("can't get user home dir: %w", err)
		}
		kubeConfigPath = path.Join(homeDir, ".kube", "config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err == nil {
		return config, nil
	}
	// fallback: use in-cluster config
	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can't access kubenetes. Tried using config from: "+
			"config parameter, %s env, homedir and InClusterConfig. Got: %w",
			kubeConfigEnvVariable, err)
	}
	return config, nil
}

func (k *KubeData) initInformers(client kubernetes.Interface) error {
	informerFactory := informers.NewSharedInformerFactory(client, syncTime)
	err := k.initNodeInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.initPodInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.initServiceInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.initReplicaSetInformer(informerFactory)
	if err != nil {
		return err
	}

	log.Debugf("starting kubernetes informers, waiting for syncronization")
	informerFactory.Start(k.stopChan)
	informerFactory.WaitForCacheSync(k.stopChan)
	log.Debugf("kubernetes informers started")

	return nil
}
