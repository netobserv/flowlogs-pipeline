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

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "net/http/pprof"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/datasource"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/k8scache"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	buildVersion       = "unknown"
	buildDate          = "unknown"
	cfgFile            string
	logLevel           string
	envPrefix          = "FLOWLOGS-PIPELINE"
	defaultLogFileName = ".flowlogs-pipeline"
	opts               config.Options
)

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Use:   "flowlogs-pipeline",
	Short: "Transform, persist and expose flow-logs as network metrics",
	Run: func(_ *cobra.Command, _ []string) {
		run()
	},
}

// initConfig use config file and ENV variables if set.
func initConfig() {
	v := viper.New()

	if cfgFile != "" {
		// Use config file from the flag.
		v.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		// Search config in home directory with name ".flowlogs-pipeline" (without extension).
		v.AddConfigPath(home)
		v.SetConfigName(defaultLogFileName)
	}

	// Read environment variables that match prefix
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

	// If a config file is found, read it in.
	cfgErr := v.ReadInConfig()

	bindFlags(rootCmd, v)

	// initialize logger
	initLogger()

	if cfgErr != nil {
		log.Errorf("Read config error: %v", cfgErr)
	}
}

func initLogger() {
	ll, err := log.ParseLevel(logLevel)
	if err != nil {
		ll = log.ErrorLevel
	}
	log.SetLevel(ll)
	log.SetFormatter(&log.TextFormatter{DisableColors: false, FullTimestamp: true, PadLevelText: true, DisableQuote: true})
}

func dumpConfig(opts *config.Options) {
	configAsJSON, err := json.MarshalIndent(opts, "", "    ")
	if err != nil {
		panic(fmt.Sprintf("error dumping config: %v", err))
	}
	fmt.Printf("Using configuration:\n%s\n", configAsJSON)
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if strings.Contains(f.Name, ".") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, ".", "_"))
			_ = v.BindEnv(f.Name, fmt.Sprintf("%s_%s", envPrefix, envVarSuffix))
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			switch val.(type) {
			case bool, uint, string, int32, int16, int8, int, uint32, uint64, int64, float64, float32, []string, []int:
				_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
			default:
				var jsonNew = jsoniter.ConfigCompatibleWithStandardLibrary
				b, err := jsonNew.Marshal(&val)
				if err != nil {
					log.Fatalf("can't parse flag %s into json with value %v got error %s", f.Name, val, err)
					return
				}
				_ = cmd.Flags().Set(f.Name, string(b))
			}
		}
	})
}

func initFlags() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is $HOME/%s)", defaultLogFileName))
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "error", "Log level: debug, info, warning, error")
	rootCmd.PersistentFlags().StringVar(&opts.Health.Address, "health.address", "0.0.0.0", "Health server address")
	rootCmd.PersistentFlags().IntVar(&opts.Health.Port, "health.port", 0, "Health server port (default: disable health server) ")
	rootCmd.PersistentFlags().IntVar(&opts.Profile.Port, "profile.port", 0, "Go pprof tool port (default: disabled)")
	rootCmd.PersistentFlags().StringVar(&opts.K8sCacheServer.Address, "k8scache.address", "0.0.0.0", "K8s cache sync server address")
	rootCmd.PersistentFlags().IntVar(&opts.K8sCacheServer.Port, "k8scache.port", 0, "K8s cache sync server port (default: disabled)")
	rootCmd.PersistentFlags().BoolVar(&opts.K8sCacheServer.TLSEnabled, "k8scache.tls-enabled", false, "Enable TLS for K8s cache sync server")
	rootCmd.PersistentFlags().StringVar(&opts.K8sCacheServer.TLSCertPath, "k8scache.tls-cert-path", "", "Path to TLS server certificate")
	rootCmd.PersistentFlags().StringVar(&opts.K8sCacheServer.TLSKeyPath, "k8scache.tls-key-path", "", "Path to TLS server private key")
	rootCmd.PersistentFlags().StringVar(&opts.K8sCacheServer.TLSCAPath, "k8scache.tls-ca-path", "", "Path to TLS CA certificate for client verification")
	rootCmd.PersistentFlags().StringVar(&opts.PipeLine, "pipeline", "", "json of config file pipeline field")
	rootCmd.PersistentFlags().StringVar(&opts.Parameters, "parameters", "", "json of config file parameters field")
	rootCmd.PersistentFlags().StringVar(&opts.DynamicParameters, "dynamicParameters", "", "json of configmap location for dynamic parameters")
	rootCmd.PersistentFlags().StringVar(&opts.MetricsSettings, "metricsSettings", "", "json for global metrics settings")
}

func main() {
	// Initialize flags (command line parameters)
	initFlags()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() {
	var (
		err          error
		mainPipeline *pipeline.Pipeline
	)

	// Initial log message
	fmt.Printf("Starting %s:\n=====\nBuild version: %s\nBuild date: %s\n\n", filepath.Base(os.Args[0]), buildVersion, buildDate)

	// Dump configuration
	dumpConfig(&opts)

	cfg, err := config.ParseConfig(&opts)
	if err != nil {
		log.Errorf("error in parsing config file: %v", err)
		os.Exit(1)
	}

	// Setup (threads) exit manager
	utils.SetupElegantExit()
	promServer := prometheus.InitializePrometheus(&cfg.MetricsSettings)

	// Enable k8scache mode if configured (disables local informers to save resources)
	if opts.K8sCacheServer.Port > 0 {
		kubernetes.SetK8sCacheEnabled(true)
	}

	// Create new flows pipeline
	mainPipeline, err = pipeline.NewPipeline(&cfg)
	if err != nil {
		log.Errorf("failed to initialize pipeline: %s", err)
		os.Exit(1)
	}

	if opts.Profile.Port != 0 {
		go func() {
			log.WithField("port", opts.Profile.Port).Info("starting PProf HTTP listener")
			log.WithError(http.ListenAndServe(fmt.Sprintf(":%d", opts.Profile.Port), nil)).
				Error("PProf HTTP listener stopped working")
		}()
	}

	// Start health report server
	var healthServer *http.Server
	if opts.Health.Port > 0 {
		healthServer = operational.NewHealthServer(&opts, mainPipeline.IsAlive, mainPipeline.IsReady)
	}

	// Start K8s cache server
	var grpcServer *grpc.Server
	if opts.K8sCacheServer.Port > 0 {
		grpcServer = startK8sCacheServer(&opts.K8sCacheServer)
	}

	// Starts the flows pipeline
	mainPipeline.Run()

	if promServer != nil {
		_ = promServer.Shutdown(context.Background())
	}
	if healthServer != nil {
		_ = healthServer.Shutdown(context.Background())
	}
	if grpcServer != nil {
		log.Info("stopping K8s cache sync server")
		grpcServer.GracefulStop()
	}

	// Give all threads a chance to exit and then exit the process
	time.Sleep(time.Second)
	log.Debugf("exiting main run")
	os.Exit(0)
}

// startK8sCacheServer initializes and starts the gRPC server for K8s cache synchronization
// Returns nil if the datasource is not available (e.g., no kubernetes enrichment configured)
func startK8sCacheServer(cfg *config.K8sCacheServer) *grpc.Server {
	// Check if kubernetes datasource is available
	ds := kubernetes.GetDatasource()
	if ds == nil {
		log.Warn("K8s cache server requested but kubernetes datasource not initialized. " +
			"Make sure kubernetes enrichment is configured in the pipeline.")
		return nil
	}

	// Attach a Kubernetes store so the cache server can apply received updates; enrichment will use it for lookups
	ds.SetKubernetesStore(datasource.NewKubernetesStore())

	// Create cache server
	cacheServer := k8scache.NewKubernetesCacheServer(ds)

	// Create gRPC server with optional TLS
	var grpcServer *grpc.Server
	if cfg.TLSEnabled {
		tlsConfig, err := createServerTLSConfig(cfg)
		if err != nil {
			log.WithError(err).Fatal("failed to configure TLS for K8s cache server")
			return nil
		}
		grpcServer = grpc.NewServer(grpc.Creds(tlsConfig))
		log.Info("K8s cache server TLS enabled")
	} else {
		grpcServer = grpc.NewServer()
		log.Warn("K8s cache server TLS disabled - connections are insecure (not recommended for production)")
	}
	k8scache.RegisterKubernetesCacheServiceServer(grpcServer, cacheServer)

	// Start listening
	address := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.WithError(err).WithField("address", address).Fatal("failed to start K8s cache server")
		return nil
	}

	// Start server in background
	go func() {
		log.WithField("address", address).Info("starting K8s cache sync server")
		if err := grpcServer.Serve(listener); err != nil {
			log.WithError(err).Error("K8s cache sync server stopped with error")
		}
	}()

	return grpcServer
}

// createServerTLSConfig creates TLS credentials for the gRPC server
func createServerTLSConfig(cfg *config.K8sCacheServer) (credentials.TransportCredentials, error) {
	// Load server certificate and private key
	if cfg.TLSCertPath == "" || cfg.TLSKeyPath == "" {
		return nil, fmt.Errorf("TLS enabled but cert/key paths not provided")
	}

	cert, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server cert/key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert, // Default: no client cert required
	}

	// If CA is provided, require and verify client certificates
	if cfg.TLSCAPath != "" {
		caCert, err := os.ReadFile(cfg.TLSCAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert")
		}

		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		log.Info("K8s cache server: mutual TLS enabled (client certificates required)")
	}

	return credentials.NewTLS(tlsConfig), nil
}
