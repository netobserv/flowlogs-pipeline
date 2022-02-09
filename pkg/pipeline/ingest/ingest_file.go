/*
 * Copyright (C) 2021 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *	 http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package ingest

import (
	"bufio"
	"fmt"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

type ingestFile struct {
	fileName string
	exitChan chan bool
}

const delaySeconds = 10

// Ingest ingests entries from a file and resends the same data every delaySeconds seconds
func (r *ingestFile) Ingest(process ProcessFunction) {
	lines := make([]interface{}, 0)
	file, err := os.Open(r.fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		log.Debugf("%s", text)
		lines = append(lines, text)
	}
	log.Debugf("Ingesting %d log lines from %s", len(lines), r.fileName)
	switch config.Opt.PipeLine.Ingest.Type {
	case "file":
		process(lines)
	case "file_loop":
		// loop forever
		ticker := time.NewTicker(time.Duration(delaySeconds) * time.Second)
		for {
			select {
			case <-r.exitChan:
				log.Debugf("exiting ingestFile because of signal")
				return
			case <-ticker.C:
				log.Debugf("ingestFile; for loop; before process")
				process(lines)
			}
		}
	}
}

// NewIngestFile create a new ingester
func NewIngestFile() (Ingester, error) {
	log.Debugf("entering NewIngestFile")
	if config.Opt.PipeLine.Ingest.File.Filename == "" {
		return nil, fmt.Errorf("ingest filename not specified")
	}

	log.Infof("input file name = %s", config.Opt.PipeLine.Ingest.File.Filename)

	ch := make(chan bool, 1)
	utils.RegisterExitChannel(ch)
	return &ingestFile{
		fileName: config.Opt.PipeLine.Ingest.File.Filename,
		exitChan: ch,
	}, nil
}
