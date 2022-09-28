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

type Info struct {
	Type            string
	Name            string
	Namespace       string
	Labels          map[string]string
	OwnerReferences []metav1.OwnerReference
	Owner           Owner
	HostName        string
	HostIP          string
	IPs             []string
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
			case typeNode:
				node := objs[0].(*v1.Node)
				info = &Info{
					Type:      typeNode,
					Name:      node.Name,
					Namespace: node.Namespace,
					Labels:    node.Labels,
				}
			case typeService:
				service := objs[0].(*v1.Service)
				info = &Info{
					Type:      typeService,
					Name:      service.Name,
					Namespace: service.Namespace,
					Labels:    service.Labels,
				}
			}

			info.Owner = k.getOwner(info)
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
			return objs[0].(*v1.Node).Name
		}
	}
	return ""
}

func (k *KubeData) NewNodeInformer(informerFactory informers.SharedInformerFactory) error {
	nodes := informerFactory.Core().V1().Nodes().Informer()
	err := nodes.AddIndexers(map[string]cache.IndexFunc{
		IndexIP: func(obj interface{}) ([]string, error) {
			node := obj.(*v1.Node)
			ips := make([]string, 0, len(node.Status.Addresses))
			for _, address := range node.Status.Addresses {
				ip := net.ParseIP(address.Address)
				if ip != nil {
					ips = append(ips, ip.String())
				}
			}
			// CNI-dependent logic (must work regardless of whether the CNI is installed)
			ips = cni.AddOvnIPs(ips, node)

			return ips, nil
		},
	})
	k.ipInformers[typeNode] = nodes
	return err
}

func (k *KubeData) NewPodInformer(informerFactory informers.SharedInformerFactory) error {
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
			Type:            typePod,
			Name:            pod.Name,
			Namespace:       pod.Namespace,
			Labels:          pod.Labels,
			OwnerReferences: pod.OwnerReferences,
			HostIP:          pod.Status.HostIP,
			IPs:             ips,
		}, nil
	}); err != nil {
		return fmt.Errorf("can't set pods transform: %w", err)
	}
	err := pods.AddIndexers(map[string]cache.IndexFunc{
		IndexIP: func(obj interface{}) ([]string, error) {
			return obj.(*Info).IPs, nil
		},
	})
	if err != nil {
		return fmt.Errorf("can't add %s indexer to Pods informer: %w", IndexIP, err)
	}
	k.ipInformers[typePod] = pods
	return err
}

func (k *KubeData) NewServiceInformer(informerFactory informers.SharedInformerFactory) error {
	services := informerFactory.Core().V1().Services().Informer()
	err := services.AddIndexers(map[string]cache.IndexFunc{
		IndexIP: func(obj interface{}) ([]string, error) {
			service := obj.(*v1.Service)
			ips := service.Spec.ClusterIPs
			if service.Spec.ClusterIP == v1.ClusterIPNone {
				return []string{}, nil
			}
			return ips, nil
		},
	})

	k.ipInformers[typeService] = services
	return err
}

func (k *KubeData) NewReplicaSetInformer(informerFactory informers.SharedInformerFactory) error {
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
	err := k.NewNodeInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.NewPodInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.NewServiceInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.NewReplicaSetInformer(informerFactory)
	if err != nil {
		return err
	}

	log.Debugf("starting kubernetes informers, waiting for syncronization")
	informerFactory.Start(k.stopChan)
	informerFactory.WaitForCacheSync(k.stopChan)
	log.Debugf("kubernetes informers started")

	return nil
}
