/*
 * Copyright (C) 2024 Red Hat, Inc.
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

package k8scache

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// DiscoveryConfig holds configuration for processor discovery
type DiscoveryConfig struct {
	// Kubeconfig path (empty for in-cluster config)
	Kubeconfig string
	// ProcessorSelector is the label selector for FLP processor pods (e.g., "app=flowlogs-pipeline")
	ProcessorSelector string
	// ProcessorPort is the port where FLP processors listen for gRPC
	ProcessorPort int
	// ResyncInterval is how often to rediscover processors (in seconds)
	ResyncInterval int
	// ProcessorServiceName is the headless service name for processor pods (optional).
	// If set, discovery will use DNS names (e.g., pod-name.service-name.namespace.svc.cluster.local)
	// instead of pod IPs. This is required for proper TLS certificate validation.
	ProcessorServiceName string
}

// StartProcessorDiscovery periodically discovers FLP processor pods and connects the client to them.
// It runs in a loop until the context is cancelled, discovering processors at the configured interval.
//
// The discovery process:
// 1. Lists pods matching ProcessorSelector in the current namespace (from POD_NAMESPACE env var)
// 2. Filters for running pods with assigned IPs
// 3. Connects the client to each discovered processor (idempotent - won't duplicate connections)
//
// This function blocks until ctx is cancelled. Run it in a goroutine for background discovery.
func StartProcessorDiscovery(ctx context.Context, client *Client, cfg DiscoveryConfig) error {
	// Validate ResyncInterval before doing any work
	if cfg.ResyncInterval <= 0 {
		return fmt.Errorf("invalid ResyncInterval: %d (must be positive)", cfg.ResyncInterval)
	}

	// Get Kubernetes client
	k8sConfig, err := getK8sConfig(cfg.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to get k8s config for processor discovery: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	ticker := time.NewTicker(time.Duration(cfg.ResyncInterval) * time.Second)
	defer ticker.Stop()

	// Immediate first run
	discoverAndConnect(ctx, clientset, client, cfg)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			discoverAndConnect(ctx, clientset, client, cfg)
		}
	}
}

// discoverAndConnect discovers FLP processor pods and connects to them
// Also removes connections to processors that no longer exist (e.g., pods that restarted with new IPs)
func discoverAndConnect(ctx context.Context, clientset *kubernetes.Clientset, client *Client, cfg DiscoveryConfig) {
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: cfg.ProcessorSelector,
	})
	if err != nil {
		log.WithError(err).Error("failed to list processor pods")
		return
	}

	log.WithField("num_pods", len(pods.Items)).Debug("discovered processor pods")

	// Track discovered addresses in this cycle
	discoveredAddresses := make(map[string]bool)

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		if pod.Status.PodIP == "" {
			continue
		}

		// Build address: prefer DNS name (for TLS) if service name is configured, otherwise use IP
		var address string
		if cfg.ProcessorServiceName != "" {
			// Use DNS name: <pod-name>.<service-name>.<namespace>.svc.cluster.local:port
			// This is required for proper TLS certificate validation
			dnsName := fmt.Sprintf("%s.%s.%s.svc.cluster.local", pod.Name, cfg.ProcessorServiceName, namespace)
			address = fmt.Sprintf("%s:%d", dnsName, cfg.ProcessorPort)
			log.WithFields(log.Fields{
				"pod":      pod.Name,
				"dns_name": dnsName,
			}).Debug("using DNS name for processor connection (TLS-friendly)")
		} else {
			// Fallback to IP address (may cause TLS verification issues)
			address = fmt.Sprintf("%s:%d", pod.Status.PodIP, cfg.ProcessorPort)
		}

		// Mark this address as discovered
		discoveredAddresses[address] = true

		// AddProcessorWithTimeout is idempotent (won't duplicate if already connected)
		// Use a 10-second timeout to avoid blocking the discovery loop for too long
		if err := client.AddProcessorWithTimeout(address, 10*time.Second); err != nil {
			log.WithError(err).WithField("pod", pod.Name).Error("failed to connect to processor")
		}
	}

	// Remove connections to processors that are no longer discovered
	// This handles cases where pods restart with new IPs or are deleted
	client.RemoveStaleProcessors(discoveredAddresses)
}

// getK8sConfig returns the Kubernetes client config (in-cluster or from kubeconfig)
func getK8sConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
