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

package api

// WriteFlowBuffer configures the in-memory ring buffer of enriched flows and its query API.
// When this write stage is absent from the pipeline, no RAM buffer or query listener is started.
type WriteFlowBuffer struct {
	// MaxEntries is the per-instance ring buffer capacity. Oldest entries are evicted first.
	MaxEntries int `yaml:"maxEntries,omitempty" json:"maxEntries,omitempty" doc:"maximum number of enriched flows retained per FLP instance (default: 50000)"`
	// QueryListenAddress is the bind address for the flowBuffer HTTP API (default: ":9200").
	QueryListenAddress string `yaml:"queryListenAddress,omitempty" json:"queryListenAddress,omitempty" doc:"listen address for flowBuffer HTTP query API (default: :9200)"`
	// QueryTimeout bounds local and peer queries (default: 2s).
	QueryTimeout Duration `yaml:"queryTimeout,omitempty" json:"queryTimeout,omitempty" doc:"timeout for buffer and peer queries (default: 2s)"`
	// PeerPort is the port used when discovering sibling pods via EndpointSlice/Endpoints (default: same as listen port).
	PeerPort int `yaml:"peerPort,omitempty" json:"peerPort,omitempty" doc:"port for peer local queries (default: derived from queryListenAddress)"`
	// PeerURLs is an explicit list of peer base URLs for tests / non-K8s (e.g. http://127.0.0.1:9201).
	// When set, Kubernetes discovery is skipped.
	PeerURLs []string `yaml:"peerURLs,omitempty" json:"peerURLs,omitempty" doc:"explicit peer base URLs (skips Kubernetes discovery; for tests)"`
	// ServiceName is the Kubernetes Service name whose EndpointSlice/Endpoints list sibling pods.
	ServiceName string `yaml:"serviceName,omitempty" json:"serviceName,omitempty" doc:"Kubernetes Service name for peer discovery via EndpointSlice/Endpoints"`
	// Namespace overrides the namespace for peer discovery (default: POD_NAMESPACE env).
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" doc:"Kubernetes namespace for peer discovery (default: POD_NAMESPACE)"`
	// KubeConfigPath is an optional kubeconfig path (empty = in-cluster).
	KubeConfigPath string `yaml:"kubeConfigPath,omitempty" json:"kubeConfigPath,omitempty" doc:"path to kubeconfig (empty for in-cluster)"`
}
