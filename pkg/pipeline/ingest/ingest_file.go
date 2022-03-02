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
	"os"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
)

type IngestFile struct {
	params      config.Ingest
	exitChan    chan bool
	PrevRecords []interface{}
}

const delaySeconds = 10

// Ingest ingests entries from a file and resends the same data every delaySeconds seconds
func (ingestF *IngestFile) Ingest(process ProcessFunction) {
	lines := make([]interface{}, 0)
	file, err := os.Open(ingestF.params.File.Filename)
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
	log.Debugf("Ingesting %d log lines from %s", len(lines), ingestF.params.File.Filename)
	switch ingestF.params.Type {
	case "file":
		ingestF.PrevRecords = lines
		process(lines)
	case "file_loop":
		// loop forever
		ticker := time.NewTicker(time.Duration(delaySeconds) * time.Second)
		for {
			select {
			case <-ingestF.exitChan:
				log.Debugf("exiting ingestFile because of signal")
				return
			case <-ticker.C:
				log.Debugf("ingestFile; for loop; before process")
				ingestF.PrevRecords = lines
				process(lines)
			}
		}
	}
}

// NewIngestFile create a new ingester
func NewIngestFile(params config.StageParam) (Ingester, error) {
	log.Debugf("entering NewIngestFile")
	if params.Ingest.File.Filename == "" {
		return nil, fmt.Errorf("ingest filename not specified")
	}

	log.Debugf("input file name = %s", params.Ingest.File.Filename)

	ch := make(chan bool, 1)
	utils.RegisterExitChannel(ch)
	return &IngestFile{
		params:   params.Ingest,
		exitChan: ch,
	}, nil
}
