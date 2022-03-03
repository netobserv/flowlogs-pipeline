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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/health"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	BuildVersion       string
	BuildDate          string
	cfgFile            string
	logLevel           string
	envPrefix          = "FLOWLOGS-PIPILNE"
	defaultLogFileName = ".flowlogs-pipeline"
)

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Use:   "flowlogs-pipeline",
	Short: "Expose network flow-logs from metrics",
	Run: func(cmd *cobra.Command, args []string) {
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
	_ = v.ReadInConfig()

	bindFlags(rootCmd, v)

	// initialize logger
	initLogger()
}

func initLogger() {
	ll, err := log.ParseLevel(logLevel)
	if err != nil {
		ll = log.ErrorLevel
	}
	log.SetLevel(ll)
	log.SetFormatter(&log.TextFormatter{DisableColors: false, FullTimestamp: true, PadLevelText: true})
}

func dumpConfig() {
	configAsJSON, _ := json.MarshalIndent(config.Opt, "", "    ")
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
	rootCmd.PersistentFlags().StringVar(&config.Opt.Health.Port, "health.port", "8080", "Health server port")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine, "pipeline", "", "json of config file pipeline field")
	rootCmd.PersistentFlags().StringVar(&config.Opt.Parameters, "parameters", "", "json of config file parameters field")
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
	fmt.Printf("Starting %s:\n=====\nBuild Version: %s\nBuild Date: %s\n\n",
		filepath.Base(os.Args[0]), BuildVersion, BuildDate)

	// Dump configuration
	dumpConfig()

	err = config.ParseConfig()
	if err != nil {
		log.Errorf("error in parsing config file: %v", err)
		os.Exit(1)
	}

	// Setup (threads) exit manager
	utils.SetupElegantExit()

	// Create new flows pipeline
	mainPipeline, err = pipeline.NewPipeline()
	if err != nil {
		log.Fatalf("failed to initialize pipeline %s", err)
		os.Exit(1)
	}

	// Start health report server
	health.NewHealthServer(mainPipeline)

	// Starts the flows pipeline
	mainPipeline.Run()

	// Give all threads a chance to exit and then exit the process
	time.Sleep(time.Second)
	log.Debugf("exiting main run")
	os.Exit(0)

}
