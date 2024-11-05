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
	"encoding/json"
	"fmt"
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
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
	rootCmd.PersistentFlags().StringVar(&opts.Health.Port, "health.port", "8080", "Health server port")
	rootCmd.PersistentFlags().IntVar(&opts.Profile.Port, "profile.port", 0, "Go pprof tool port (default: disabled)")
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
	healthServer := operational.NewHealthServer(&opts, mainPipeline.IsAlive, mainPipeline.IsReady)

	// Starts the flows pipeline
	mainPipeline.Run()

	if promServer != nil {
		_ = promServer.Shutdown(context.Background())
	}
	_ = healthServer.Shutdown(context.Background())

	// Give all threads a chance to exit and then exit the process
	time.Sleep(time.Second)
	log.Debugf("exiting main run")
	os.Exit(0)
}
