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
	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	Version            string
	cfgFile            string
	logLevel           string
	envPrefix          = "FLOWLOGS2METRICS"
	defaultLogFileName = ".flowlogs2metrics"
)

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Use:   "flowlogs2metrics",
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
		// Search config in home directory with name ".flowlogs2metrics" (without extension).
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
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Ingest.Type, "pipeline.ingest.type", "", "Ingest type: file, collector,file_loop (required)")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Ingest.File.Filename, "pipeline.ingest.file.filename", "", "Ingest filename (file)")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Ingest.Collector, "pipeline.ingest.collector", "", "Ingest collector API")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Ingest.Kafka, "pipeline.ingest.kafka", "", "Ingest Kafka API")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Decode.Aws, "pipeline.decode.aws", "", "aws fields")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Decode.Type, "pipeline.decode.type", "", "Decode type: aws, json, none")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Transform, "pipeline.transform", "[{\"type\": \"none\"}]", "Transforms (list) API")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Extract.Type, "pipeline.extract.type", "", "Extract type: aggregates, none")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Extract.Aggregates, "pipeline.extract.aggregates", "", "Aggregates (see docs)")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Encode.Type, "pipeline.encode.type", "", "Encode type: prom, json, none")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Write.Type, "pipeline.write.type", "", "Write type: stdout, none")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Write.Loki, "pipeline.write.loki", "", "Loki write API")
	rootCmd.PersistentFlags().StringVar(&config.Opt.PipeLine.Encode.Prom, "pipeline.encode.prom", "", "Prometheus encode API")

	_ = rootCmd.MarkPersistentFlagRequired("pipeline.ingest.type")
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

	// Starting log message
	fmt.Printf("%s starting - version [%s]\n\n", filepath.Base(os.Args[0]), Version)
	// Dump the configuration
	dumpConfig()

	utils.SetupElegantExit()

	// creating a new pipeline
	mainPipeline, err = pipeline.NewPipeline()
	if err != nil {
		log.Fatalf("failed to initialize pipeline %s", err)
		os.Exit(1)
	}

	mainPipeline.Run()

	// give all threads a chance to exit and then exit the process
	time.Sleep(time.Second)
	log.Debugf("exiting main run")
	os.Exit(0)

}
