/*
 * Copyright (C) 2026 NetObserv Authors.
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

package flowbuffer

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var plog = logrus.WithField("component", "write.FlowBuffer.peers")

// EndpointSlicePeers discovers sibling pods via EndpointSlice (fallback: Endpoints).
type EndpointSlicePeers struct {
	Clientset   kubernetes.Interface
	Namespace   string
	ServiceName string
	PeerPort    int
	SelfIP      string
}

// NewEndpointSlicePeers builds a PeerLister using in-cluster or kubeconfig credentials.
func NewEndpointSlicePeers(namespace, serviceName string, peerPort int, kubeConfigPath string) (*EndpointSlicePeers, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName is required for Kubernetes peer discovery")
	}
	if namespace == "" {
		namespace = os.Getenv("POD_NAMESPACE")
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required for peer discovery (set namespace or POD_NAMESPACE)")
	}
	cfg, err := restConfig(kubeConfigPath)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &EndpointSlicePeers{
		Clientset:   cs,
		Namespace:   namespace,
		ServiceName: serviceName,
		PeerPort:    peerPort,
		SelfIP:      os.Getenv("POD_IP"),
	}, nil
}

func restConfig(kubeConfigPath string) (*rest.Config, error) {
	if kubeConfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}
	return rest.InClusterConfig()
}

// ListPeers returns http://ip:port base URLs for ready endpoints, excluding self.
func (e *EndpointSlicePeers) ListPeers(ctx context.Context) ([]string, error) {
	urls, err := e.fromEndpointSlices(ctx)
	if err == nil {
		return urls, nil
	}
	plog.WithError(err).Debug("EndpointSlice discovery failed; trying Endpoints")
	return e.fromEndpoints(ctx)
}

func (e *EndpointSlicePeers) fromEndpointSlices(ctx context.Context) ([]string, error) {
	slices, err := e.Clientset.DiscoveryV1().EndpointSlices(e.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + e.ServiceName,
	})
	if err != nil {
		return nil, err
	}
	var urls []string
	seen := map[string]struct{}{}
	for i := range slices.Items {
		urls = append(urls, e.urlsFromSlice(&slices.Items[i], seen)...)
	}
	return urls, nil
}

func (e *EndpointSlicePeers) urlsFromSlice(slice *discoveryv1.EndpointSlice, seen map[string]struct{}) []string {
	port := e.PeerPort
	if port == 0 {
		for _, p := range slice.Ports {
			if p.Port != nil {
				port = int(*p.Port)
				break
			}
		}
	}
	if port == 0 {
		port = 9200
	}
	var urls []string
	for _, ep := range slice.Endpoints {
		if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
			continue
		}
		for _, addr := range ep.Addresses {
			if e.SelfIP != "" && addr == e.SelfIP {
				continue
			}
			base := fmt.Sprintf("http://%s", net.JoinHostPort(addr, strconv.Itoa(port)))
			if _, ok := seen[base]; ok {
				continue
			}
			seen[base] = struct{}{}
			urls = append(urls, base)
		}
	}
	return urls
}

func (e *EndpointSlicePeers) fromEndpoints(ctx context.Context) ([]string, error) {
	ep, err := e.Clientset.CoreV1().Endpoints(e.Namespace).Get(ctx, e.ServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	port := e.PeerPort
	seen := map[string]struct{}{}
	var urls []string
	for _, subset := range ep.Subsets {
		p := port
		if p == 0 {
			for _, sp := range subset.Ports {
				p = int(sp.Port)
				break
			}
		}
		if p == 0 {
			p = 9200
		}
		for _, addr := range subset.Addresses {
			if e.SelfIP != "" && addr.IP == e.SelfIP {
				continue
			}
			base := fmt.Sprintf("http://%s", net.JoinHostPort(addr.IP, strconv.Itoa(p)))
			if _, ok := seen[base]; ok {
				continue
			}
			seen[base] = struct{}{}
			urls = append(urls, base)
		}
	}
	return urls, nil
}
