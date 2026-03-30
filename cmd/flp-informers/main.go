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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	commit    = "unknown"
	envPrefix = "FLP_INFORMERS"
)

type options struct {
	Kubeconfig           string
	LogLevel             string
	ProcessorSelector    string // Label selector for FLP processors (e.g., "app=flowlogs-pipeline")
	ProcessorPort        int    // Port where FLP processors listen for gRPC (k8scache.port)
	ProcessorServiceName string // Headless service name for DNS-based discovery (optional, required for TLS)
	ResyncInterval       int    // Interval in seconds to rediscover processors
	// TLS configuration for gRPC client
	TLSEnabled         bool
	TLSCertPath        string
	TLSKeyPath         string
	TLSCAPath          string
	TLSServerName      string
	InsecureSkipVerify bool
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
	rootCmd.PersistentFlags().StringVar(&opts.ProcessorServiceName, "processor-service-name", "", "Headless service name for DNS-based discovery (required for TLS)")
	rootCmd.PersistentFlags().IntVar(&opts.ResyncInterval, "resync-interval", 60, "Interval in seconds to rediscover processors")
	// TLS configuration
	rootCmd.PersistentFlags().BoolVar(&opts.TLSEnabled, "tls-enabled", false, "Enable TLS for gRPC connections to processors")
	rootCmd.PersistentFlags().StringVar(&opts.TLSCertPath, "tls-cert-path", "", "Path to TLS client certificate")
	rootCmd.PersistentFlags().StringVar(&opts.TLSKeyPath, "tls-key-path", "", "Path to TLS client private key")
	rootCmd.PersistentFlags().StringVar(&opts.TLSCAPath, "tls-ca-path", "", "Path to TLS CA certificate for server verification")
	rootCmd.PersistentFlags().StringVar(&opts.TLSServerName, "tls-server-name", "", "Expected server name for TLS verification (e.g., flowlogs-pipeline.namespace.svc)")
	rootCmd.PersistentFlags().BoolVar(&opts.InsecureSkipVerify, "tls-insecure-skip-verify", false, "Skip TLS certificate verification (not recommended for production)")
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
	clientConfig := k8scache.ClientConfig{
		ProcessorID:        processorID,
		TLSEnabled:         opts.TLSEnabled,
		TLSCertPath:        opts.TLSCertPath,
		TLSKeyPath:         opts.TLSKeyPath,
		TLSCAPath:          opts.TLSCAPath,
		TLSServerName:      opts.TLSServerName,
		InsecureSkipVerify: opts.InsecureSkipVerify,
	}
	grpcClient := k8scache.NewClient(&clientConfig)

	if opts.TLSEnabled {
		log.Info("TLS enabled for gRPC connections to processors")
		// Warn if neither TLSServerName nor ProcessorServiceName is set (may cause TLS verification issues)
		if opts.TLSServerName == "" && opts.ProcessorServiceName == "" {
			log.Warn("TLS enabled but neither --tls-server-name nor --processor-service-name is set. " +
				"TLS verification may fail when connecting by IP. Consider setting one of these options.")
		}
	} else {
		log.Warn("TLS disabled - connections to processors are insecure (not recommended for production)")
	}
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
	handler := k8scache.NewEventHandler(grpcClient)
	if err := inf.AddEventHandler(handler); err != nil {
		log.WithError(err).Fatal("failed to add event handlers")
	}
	log.Info("Informer event handlers registered for cache sync")

	// Start processor discovery in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	discoveryConfig := k8scache.DiscoveryConfig{
		Kubeconfig:           opts.Kubeconfig,
		ProcessorSelector:    opts.ProcessorSelector,
		ProcessorPort:        opts.ProcessorPort,
		ProcessorServiceName: opts.ProcessorServiceName,
		ResyncInterval:       opts.ResyncInterval,
	}

	go func() {
		if err := k8scache.StartProcessorDiscovery(ctx, grpcClient, discoveryConfig); err != nil {
			log.WithError(err).Error("processor discovery stopped")
		}
	}()

	log.Info("flp-informers started - pushing incremental updates only (no initial snapshot)")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutdown signal received, stopping...")
}
