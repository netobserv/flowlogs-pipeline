/*
 * Copyright (C) 2023 IBM, Inc.
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
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

type Options struct {
	srcFolder               string
	tgtFolder               string
	timeBetweenMeasurements time.Duration
	timeToRun               time.Duration
}

var opts Options

const (
	defaultSrcDir       = "contrib/measurements"
	defaultTgtDir       = "/tmp/perfmeasurements"
	defaultTick         = 2 * time.Second
	defaultTimeToRun    = 20 * time.Second
	definitionExt       = ".yaml"
	resultExt           = ".csv"
	flpExec             = "flowlogs-pipeline"
	resultsFolderPrefix = "perf_"
)

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Use:   "perfmeasurements",
	Short: "Run performance measurements on specified config files",
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func initFlags() {
	rootCmd.PersistentFlags().StringVar(&opts.srcFolder, "srcFolder", defaultSrcDir, "source folder")
	rootCmd.PersistentFlags().StringVar(&opts.tgtFolder, "tgtFolder", defaultTgtDir, "target folder")
	rootCmd.PersistentFlags().DurationVar(&opts.timeBetweenMeasurements, "timeBetweenMeasurements", defaultTick, "time between measurements")
	rootCmd.PersistentFlags().DurationVar(&opts.timeToRun, "timeToRun", defaultTimeToRun, "time to run each test")
}

func printFlags() {
	fmt.Printf("srcFolder = %s \n", opts.srcFolder)
	fmt.Printf("tgtFolder = %s \n", opts.tgtFolder)
	fmt.Printf("timeBetweenMeasurements = %v \n", opts.timeBetweenMeasurements)
	fmt.Printf("timeToRun = %v \n", opts.timeToRun)
}

func printFilePaths(filePaths []string) {
	fmt.Printf("filepaths of configuration files: \n")
	for _, f := range filePaths {
		fmt.Printf("%s \n", f)
	}
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
	// Dump the configuration
	printFlags()
	filePaths := getYamlFileNames(opts.srcFolder, "")
	printFilePaths(filePaths)
	tgtFolder := opts.tgtFolder + "/" + resultsFolderPrefix + time.Now().Format(time.RFC3339)
	err := createTargetFolder(tgtFolder)
	if err != nil {
		fmt.Printf("could not create target folder; err = %v, dirName = %s \n", err, opts.tgtFolder)
		os.Exit(1)
	}
	runMeasurements(opts.srcFolder, filePaths, tgtFolder)
}

func getYamlFileNames(rootPath string, prefix string) []string {
	var files []string

	newRootPath := filepath.Join(rootPath, prefix)
	dirEntries, err := os.ReadDir(newRootPath)
	if err != nil {
		fmt.Printf("could not read directory; err = %v, dirName = %s \n", err, rootPath)
		return nil
	}
	for _, f := range dirEntries {
		fMode := f.Type()
		fName := f.Name()
		if fMode.IsRegular() && filepath.Ext(fName) == definitionExt {
			if err != nil {
				fmt.Printf("could not obtain file path name; err = %v, fileName = %s \n", err, f.Name())
				return nil
			}
			fileName := filepath.Join(prefix, fName)
			files = append(files, fileName)
		}
		if fMode.IsDir() {
			fPath := filepath.Join(prefix, fName)
			subDirFiles := getYamlFileNames(rootPath, fPath)
			files = append(files, subDirFiles...)
		}
	}
	return files
}

func createTargetFolder(folderName string) error {
	err := os.MkdirAll(folderName, 0755)
	if err != nil {
		log.Debugf("os.MkdirAll err: %v ", err)
		return err
	}
	return nil
}

type outputStruct struct {
	timeFromStart float64
	cpu           float64
	memory        float64
	nFlows        float64
	nProm         float64
}

func runMeasurements(srcFolder string, filePaths []string, tgtFolder string) {
	cwd, _ := os.Getwd()
	flp := cwd + "/" + flpExec
	for _, fPath := range filePaths {
		fmt.Printf("running measurements on %s \n", fPath)
		fullFilePath := filepath.Join(srcFolder, fPath)
		fmt.Printf("fullFilePath = %s \n", fullFilePath)
		cmd := exec.Command(flp, "--config", fullFilePath)
		if err := cmd.Start(); err != nil {
			fmt.Println("Error: ", err)
			continue
		}
		startTime := time.Now()
		fmt.Printf("start time = %s \n", startTime.Format(time.RFC3339))
		ticker := time.NewTicker(opts.timeBetweenMeasurements)
		done := make(chan bool)

		// create results file
		fileName := filepath.Join(tgtFolder, fPath)
		// change the file extension
		fileName = fileName[:len(fileName)-len(filepath.Ext(fileName))] + resultExt
		fmt.Printf("output file name = %s \n", fileName)
		f, err := createTargetFile(fileName)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}
		dw := bufio.NewWriter(f)

		go func() {
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					metrics, err := collectMetrics()
					if err != nil {
						continue
					}
					currentTime := time.Now()
					timeFromStart := currentTime.Sub(startTime)
					metrics.timeFromStart = timeFromStart.Seconds()
					l := fmt.Sprintf("%f,%f,%f,%f,%f", metrics.timeFromStart, metrics.cpu, metrics.memory, metrics.nFlows, metrics.nProm)
					_, _ = dw.WriteString(l + "\n")
					dw.Flush()
				}
			}
		}()

		go func() {
			time.Sleep(opts.timeToRun)
			ticker.Stop()
			done <- true

			// kill the flp process
			_ = cmd.Process.Signal(unix.SIGINT)
		}()

		_ = cmd.Wait()
		f.Close()
	}
}

func collectMetrics() (outputStruct, error) {
	resp, err := http.Get("http://localhost:9102/metrics")
	if err != nil {
		fmt.Println("Error: ", err)
		return outputStruct{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: ", err)
	}
	lines := strings.Split(string(body), "\n")

	var cpu float64
	var memory float64
	var nFlows float64
	var nProm float64
	for _, line := range lines {
		if strings.HasPrefix(line, "flp_op_ingest_synthetic_flows_processed") {
			s := strings.Split(line, " ")
			nFlows, err = strconv.ParseFloat(s[1], 64)
			if err != nil {
				fmt.Printf("error converting nFlows; s = %v, err = %v \n", s, err)
				continue
			}
		}
		if strings.HasPrefix(line, "flp_op_encode_prom_metrics_reported") {
			s := strings.Split(line, " ")
			nProm, err = strconv.ParseFloat(s[1], 64)
			if err != nil {
				fmt.Printf("error converting nProm; s = %v, err = %v \n", s, err)
				continue
			}
		}
		if strings.HasPrefix(line, "process_cpu_seconds_total") {
			s := strings.Split(line, " ")
			cpu, err = strconv.ParseFloat(s[1], 64)
			if err != nil {
				fmt.Printf("error converting cpu; s = %v, err = %v \n", s, err)
				continue
			}
		}
		if strings.HasPrefix(line, "process_resident_memory_bytes") {
			s := strings.Split(line, " ")
			memory, err = strconv.ParseFloat(s[1], 64)
			if err != nil {
				fmt.Printf("error converting memory; s = %v, err = %v \n", s, err)
				continue
			}
		}
	}
	metrics := outputStruct{
		cpu:    cpu,
		memory: memory,
		nFlows: nFlows,
		nProm:  nProm,
	}
	return metrics, nil
}

func createTargetFile(fileName string) (*os.File, error) {
	dir := filepath.Dir(fileName)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		fmt.Printf("error creating output directory; err = %v \n", err)
		return nil, err
	}
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	return f, err
}
