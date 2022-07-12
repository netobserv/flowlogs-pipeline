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

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/confgen"
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
	envPrefix          = "FLP_CONFGEN"
	defaultLogFileName = ".confgen"
)

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Use:   "confgenerator",
	Short: "Generate configuration and docs from metric definitions",
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
		// Search config in home directory with name ".confgen" (without extension).
		v.AddConfigPath(home)
		v.SetConfigName(defaultLogFileName)
	}

	// Read environment variables that match prefix
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", v.ConfigFileUsed())
	}

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
	log.SetFormatter(&log.TextFormatter{DisableColors: false, FullTimestamp: true})
}

func dumpConfig() {
	configAsJSON, _ := json.MarshalIndent(confgen.Opt, "", "\t")
	log.Infof("configuration:\n%s\n", configAsJSON)
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
				var jsoniterJson = jsoniter.ConfigCompatibleWithStandardLibrary
				b, err := jsoniterJson.Marshal(&val)
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
	rootCmd.PersistentFlags().StringVar(&confgen.Opt.SrcFolder, "srcFolder", "network_definitions", "source folder")
	rootCmd.PersistentFlags().StringVar(&confgen.Opt.DestConfFile, "destConfFile", "/tmp/flowlogs-pipeline.conf.yaml", "destination configuration file")
	rootCmd.PersistentFlags().StringVar(&confgen.Opt.DestDocFile, "destDocFile", "/tmp/metrics.md", "destination documentation file (.md)")
	rootCmd.PersistentFlags().StringVar(&confgen.Opt.DestGrafanaJsonnetFolder, "destGrafanaJsonnetFolder", "/tmp/jsonnet", "destination grafana jsonnet folder")
	rootCmd.PersistentFlags().StringSliceVar(&confgen.Opt.SkipWithTags, "skipWithTags", nil, "Skip definitions with Tags")
	rootCmd.PersistentFlags().StringSliceVar(&confgen.Opt.GenerateStages, "generateStages", nil, "Produce only specified stages (ingest, transform_generic, transform_network, extract_aggregate, encode_prom, write_loki")
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
	// Initial log message
	fmt.Printf("Starting %s:\n=====\nBuild Version: %s\nBuild Date: %s\n\n",
		filepath.Base(os.Args[0]), BuildVersion, BuildDate)
	// Dump the configuration
	dumpConfig()
	// creating a new configuration generator
	confGen, err := confgen.NewConfGen()
	if err != nil {
		log.Fatalf("failed to initialize NewConfGen %s", err)
		os.Exit(1)
	}

	err = confGen.Run()
	if err != nil {
		log.Fatalf("failed to initialize NewConfGen %s", err)
		os.Exit(1)
	}
}
