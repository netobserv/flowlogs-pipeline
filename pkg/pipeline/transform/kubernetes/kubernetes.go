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
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"net"
	"os"
	"time"
)

var Data KubeData

const (
	kubeConfigEnvVariable = "KUBECONFIG"
	syncTime              = 10 * time.Minute
	IndexIP               = "byIP"
	typeNode              = "node"
	typePod               = "pod"
	typeService           = "service"
)

type KubeData struct {
	informers map[string]cache.SharedIndexInformer
	stopChan  chan struct{}
}

type Info struct {
	Type      string
	Name      string
	Namespace string
	Labels    map[string]string
}

func (k KubeData) GetInfo(ip string) (*Info, error) {
	for objType, informer := range k.informers {
		objs, err := informer.GetIndexer().ByIndex(IndexIP, ip)
		if err == nil && len(objs) > 0 {
			switch objType {
			case typePod:
				pod := objs[0].(*v1.Pod)
				return &Info{
					Type:      typePod,
					Name:      pod.Name,
					Namespace: pod.Namespace,
					Labels:    pod.Labels,
				}, nil
			case typeNode:
				node := objs[0].(*v1.Node)
				return &Info{
					Type:      typeNode,
					Name:      node.Name,
					Namespace: node.Namespace,
					Labels:    node.Labels,
				}, nil
			case typeService:
				service := objs[0].(*v1.Service)
				return &Info{
					Type:      typeService,
					Name:      service.Name,
					Namespace: service.Namespace,
					Labels:    service.Labels,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("can't find ip")
}

func (k KubeData) NewNodeInformer(informerFactory informers.SharedInformerFactory) error {
	nodes := informerFactory.Core().V1().Nodes().Informer()
	err := nodes.AddIndexers(map[string]cache.IndexFunc{
		IndexIP: func(obj interface{}) ([]string, error) {
			node := obj.(*v1.Node)
			ips := make([]string, 0, len(node.Status.Addresses))
			for _, address := range node.Status.Addresses {
				ip := net.ParseIP(address.Address); if ip != nil {
					ips = append(ips, ip.String())
				}
			}
			return ips, nil
		},
	})

	k.informers[typeNode] = nodes
	return err
}

func (k KubeData) NewPodInformer(informerFactory informers.SharedInformerFactory) error {
	pods := informerFactory.Core().V1().Pods().Informer()
	err := pods.AddIndexers(map[string]cache.IndexFunc{
		IndexIP: func(obj interface{}) ([]string, error) {
			pod := obj.(*v1.Pod)
			ips := make([]string, 0, len(pod.Status.PodIPs))
			for _, ip := range pod.Status.PodIPs {
				// ignoring host-networked Pod IPs
				if ip.IP != pod.Status.HostIP {
					ips = append(ips, ip.IP)
				}
			}
			return ips, nil
		},
	})

	k.informers[typePod] = pods
	return err
}

func (k KubeData) NewServiceInformer(informerFactory informers.SharedInformerFactory) error {
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

	k.informers[typeService] = services
	return err
}

func (k KubeData) InitFromConfig(kubeConfigPath string) error {
	var config *rest.Config
	var err error

	if kubeConfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return fmt.Errorf("can't build config from %s", kubeConfigPath)
		}
	} else {
		kubeConfigPath = os.Getenv(kubeConfigEnvVariable)
		if kubeConfigPath != "" {
			config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
			if err != nil {
				return fmt.Errorf("can't build config from %s", kubeConfigPath)
			}
		} else {
			homeDir, _ := os.UserHomeDir()
			config, err = clientcmd.BuildConfigFromFlags("", homeDir+"/.kube/config")
			if err != nil {
				// creates the in-cluster config
				config, err = rest.InClusterConfig()
				if err != nil {
					return fmt.Errorf("can't access kubenetes. Tried using config from: config parameter, %s env, homedir and InClusterConfig", kubeConfigEnvVariable)
				}
			}
		}
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

func (k KubeData) initInformers(client kubernetes.Interface) error {
	//defer close(stopChan)

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

	log.Debugf("starting kubernetes informer, waiting for syncronization")
	informerFactory.Start(k.stopChan)
	informerFactory.WaitForCacheSync(k.stopChan)
	log.Debugf("kubernetes informers started")

	return nil
}

func init() {
	Data = KubeData{
		informers: map[string]cache.SharedIndexInformer{},
		stopChan:  make(chan struct{}),
	}
}
