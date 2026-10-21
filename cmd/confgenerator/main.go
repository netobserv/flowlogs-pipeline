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

	cliConfig "github.com/netobserv/flowlogs-pipeline/pkg/cli"
	"github.com/netobserv/flowlogs-pipeline/pkg/confgen"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	buildVersion       = "unknown"
	buildDate          = "unknown"
	cfgFile            string
	logLevel           string
	envPrefix          = "FLP_CONFGEN"
	defaultLogFileName = ".confgen"
	opts               confgen.Options
)

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Use:   "confgenerator",
	Short: "Generate configuration and docs from metric definitions",
	Run: func(_ *cobra.Command, _ []string) {
		run()
	},
}

// initConfig use config file and ENV variables if set.
func initConfig() {
	path, sections, err := cliConfig.ReadConfig(cfgFile, defaultLogFileName)
	if err != nil {
		log.Fatalf("Read config error: %v", err)
	}
	if path != "" {
		fmt.Println("Using config file:", path)
	}
	err = cliConfig.ApplyConfig(rootCmd, sections, func(flagName string) string {
		suffix := flagName
		if strings.Contains(suffix, ".") {
			suffix = strings.ReplaceAll(suffix, ".", "_")
		}
		return strings.ToUpper(envPrefix + "_" + suffix)
	})
	if err != nil {
		log.Fatalf("Read config error: %v", err)
	}

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

func dumpConfig(opts *confgen.Options) {
	configAsJSON, _ := json.MarshalIndent(opts, "", "\t")
	log.Infof("configuration:\n%s\n", configAsJSON)
}

func initFlags() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is $HOME/%s)", defaultLogFileName))
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "error", "Log level: debug, info, warning, error")
	rootCmd.PersistentFlags().StringVar(&opts.SrcFolder, "srcFolder", "network_definitions", "source folder")
	rootCmd.PersistentFlags().StringVar(&opts.DestConfFile, "destConfFile", "/tmp/flowlogs-pipeline.conf.yaml", "destination configuration file")
	rootCmd.PersistentFlags().StringVar(&opts.DestDocFile, "destDocFile", "/tmp/metrics.md", "destination documentation file (.md)")
	rootCmd.PersistentFlags().StringVar(&opts.DestGrafanaJsonnetFolder, "destGrafanaJsonnetFolder", "/tmp/jsonnet", "destination grafana jsonnet folder")
	rootCmd.PersistentFlags().StringVar(&opts.DestDashboardFolder, "destDashboardFolder", "/tmp/dashboards", "destination grafana dashboard folder")
	rootCmd.PersistentFlags().StringSliceVar(&opts.SkipWithTags, "skipWithTags", nil, "Skip definitions with Tags")
	rootCmd.PersistentFlags().StringSliceVar(&opts.GenerateStages, "generateStages", nil, "Produce only specified stages (ingest, transform_generic, transform_network, extract_aggregate, encode_prom, write_loki")
	rootCmd.PersistentFlags().StringVar(&opts.GlobalMetricsPrefix, "globalMetricsPrefix", "", "Common prefix for all generated metrics, including operational ones")
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
	fmt.Printf("Starting %s:\n=====\nBuild version: %s\nBuild date: %s\n\n", filepath.Base(os.Args[0]), buildVersion, buildDate)
	// Dump the configuration
	dumpConfig(&opts)
	// creating a new configuration generator
	confGen := confgen.NewConfGen(&opts)
	err := confGen.Run()
	if err != nil {
		log.Errorf("failed to initialize NewConfGen %s", err)
		os.Exit(1)
	}
}
