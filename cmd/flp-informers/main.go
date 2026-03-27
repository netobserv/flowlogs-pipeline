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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/informers"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/k8scache"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	version   = "dev"
	commit    = "unknown"
	envPrefix = "FLP_INFORMERS"
)

type options struct {
	Kubeconfig        string
	LogLevel          string
	ProcessorSelector string // Label selector for FLP processors (e.g., "app=flowlogs-pipeline")
	ProcessorPort     int    // Port where FLP processors listen for gRPC (k8scache.port)
	ResyncInterval    int    // Interval in seconds to rediscover processors
}

var opts = options{}

var rootCmd = &cobra.Command{
	Use:   "flp-informers",
	Short: "Centralized Kubernetes informers that push cache updates to FLP processors",
	Long: `flp-informers watches Kubernetes resources (Pods, Nodes, Services) and pushes
updates to distributed FlowLogs Pipeline (FLP) processor pods via gRPC.

This reduces the load on the Kubernetes API server by having a single component
(or 1-2 replicas) query the API instead of N FLP processors.`,
	Run: run,
}

// initConfig reads environment variables that match the prefix
func initConfig() {
	v := viper.New()

	// Read environment variables that match prefix
	// Format: FLP_INFORMERS_<FLAG_NAME_WITH_UNDERSCORES>
	// Example: FLP_INFORMERS_LOG_LEVEL, FLP_INFORMERS_PROCESSOR_PORT
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

	bindFlags(rootCmd, v)

	// Initialize logger
	initLogger()
}

func initLogger() {
	lvl, err := log.ParseLevel(opts.LogLevel)
	if err != nil {
		lvl = log.ErrorLevel
	}
	log.SetLevel(lvl)
	log.SetFormatter(&log.TextFormatter{DisableColors: false, FullTimestamp: true, PadLevelText: true, DisableQuote: true})
}

// bindFlags applies environment variable overrides to flags
// This follows the same pattern as flowlogs-pipeline/main.go
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Convert flag name to env var format (e.g., "log-level" -> "LOG_LEVEL")
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			_ = v.BindEnv(f.Name, fmt.Sprintf("%s_%s", envPrefix, envVarSuffix))
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func initFlags() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&opts.Kubeconfig, "kubeconfig", "", "Path to kubeconfig file (empty = in-cluster)")
	rootCmd.PersistentFlags().StringVar(&opts.LogLevel, "log-level", "info", "Log level: debug, info, warning, error")
	rootCmd.PersistentFlags().StringVar(&opts.ProcessorSelector, "processor-selector", "app=flowlogs-pipeline", "Label selector for FLP processor pods")
	rootCmd.PersistentFlags().IntVar(&opts.ProcessorPort, "processor-port", 9090, "Port where FLP processors listen for gRPC")
	rootCmd.PersistentFlags().IntVar(&opts.ResyncInterval, "resync-interval", 60, "Interval in seconds to rediscover processors")
}

func main() {
	initFlags()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) {
	log.Infof("Starting flp-informers version=%s commit=%s", version, commit)

	// Create gRPC client
	processorID := fmt.Sprintf("flp-informers-%d", time.Now().Unix())
	grpcClient := k8scache.NewClient(processorID)
	grpcClient.Start()
	defer grpcClient.Stop()

	// Initialize Kubernetes informers
	apiConfig := &api.NetworkTransformKubeConfig{} // Empty config - will use defaults
	infConfig := informers.NewConfig(apiConfig)
	inf := &informers.Informers{}
	opMetrics := operational.NewMetrics(&config.MetricsSettings{})

	if err := inf.InitFromConfig(opts.Kubeconfig, &infConfig, opMetrics); err != nil {
		log.WithError(err).Fatal("failed to initialize informers")
	}

	log.Info("Kubernetes informers initialized and synced")

	// Setup informer event handlers to push updates via gRPC
	setupInformerHandlers(inf, grpcClient)

	// Start processor discovery
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go discoverProcessors(ctx, grpcClient)

	log.Info("flp-informers started - pushing incremental updates only (no initial snapshot)")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutdown signal received, stopping...")
}

// setupInformerHandlers attaches event handlers to informers to push updates via gRPC
func setupInformerHandlers(inf *informers.Informers, client *k8scache.Client) {
	handler := &cacheEventHandler{client: client}
	if err := inf.AddEventHandler(handler); err != nil {
		log.WithError(err).Fatal("failed to add event handlers")
	}
	log.Info("Informer event handlers registered for cache sync")
}

// cacheEventHandler implements informers.EventHandler to push updates via gRPC
type cacheEventHandler struct {
	client *k8scache.Client
}

func (h *cacheEventHandler) OnAdd(obj interface{}, isInInitialList bool) {
	if isInInitialList {
		// Skip initial list - we send full snapshot separately
		return
	}

	meta, ok := obj.(*model.ResourceMetaData)
	if !ok {
		log.Warnf("unexpected object type in OnAdd: %T", obj)
		return
	}

	if err := h.client.SendAdd([]*model.ResourceMetaData{meta}); err != nil {
		log.WithError(err).WithField("resource", meta.Name).Error("failed to send ADD")
	}
}

func (h *cacheEventHandler) OnUpdate(_, newObj interface{}) {
	meta, ok := newObj.(*model.ResourceMetaData)
	if !ok {
		log.Warnf("unexpected object type in OnUpdate: %T", newObj)
		return
	}

	if err := h.client.SendUpdate([]*model.ResourceMetaData{meta}); err != nil {
		log.WithError(err).WithField("resource", meta.Name).Error("failed to send UPDATE")
	}
}

func (h *cacheEventHandler) OnDelete(obj interface{}) {
	meta, ok := obj.(*model.ResourceMetaData)
	if !ok {
		log.Warnf("unexpected object type in OnDelete: %T", obj)
		return
	}

	if err := h.client.SendDelete([]*model.ResourceMetaData{meta}); err != nil {
		log.WithError(err).WithField("resource", meta.Name).Error("failed to send DELETE")
	}
}

// discoverProcessors periodically discovers FLP processor pods and connects to them
func discoverProcessors(ctx context.Context, client *k8scache.Client) {
	// Get in-cluster k8s client
	config, err := getK8sConfig()
	if err != nil {
		log.WithError(err).Fatal("failed to get k8s config for processor discovery")
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithError(err).Fatal("failed to create k8s clientset")
		return
	}

	ticker := time.NewTicker(time.Duration(opts.ResyncInterval) * time.Second)
	defer ticker.Stop()

	// Immediate first run
	discoverAndConnect(ctx, clientset, client)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			discoverAndConnect(ctx, clientset, client)
		}
	}
}

// discoverAndConnect discovers FLP processor pods and connects to them
func discoverAndConnect(ctx context.Context, clientset *kubernetes.Clientset, client *k8scache.Client) {
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: opts.ProcessorSelector,
	})
	if err != nil {
		log.WithError(err).Error("failed to list processor pods")
		return
	}

	log.WithField("num_pods", len(pods.Items)).Debug("discovered processor pods")

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		if pod.Status.PodIP == "" {
			continue
		}

		address := fmt.Sprintf("%s:%d", pod.Status.PodIP, opts.ProcessorPort)

		// AddProcessor is idempotent (won't duplicate if already connected)
		if err := client.AddProcessor(address); err != nil {
			log.WithError(err).WithField("pod", pod.Name).Error("failed to connect to processor")
		}
	}
}

// getK8sConfig returns the Kubernetes client config (in-cluster or from kubeconfig)
func getK8sConfig() (*rest.Config, error) {
	if opts.Kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", opts.Kubeconfig)
	}
	return rest.InClusterConfig()
}
