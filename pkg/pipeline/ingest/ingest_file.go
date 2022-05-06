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
	"os"
	"time"

	"github.com/mariomac/pipes/pkg/node"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
)

type IngestFile struct {
	params       config.File
	exitChan     <-chan struct{}
	PrevRecords  []interface{}
	TotalRecords int
}

const (
	delaySeconds = 10
	chunkLines   = 100
)

// Ingest ingests entries from a file and resends the same data every delaySeconds seconds
func (ingestF *IngestFile) Ingest(out chan<- []interface{}) {
	lines := make([]interface{}, 0)
	file, err := os.Open(ingestF.params.Filename)
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

	log.Debugf("Ingesting %d log lines from %s", len(lines), ingestF.params.Filename)
	switch ingestF.params.Type {
	case "file":
		ingestF.PrevRecords = lines
		ingestF.TotalRecords = len(lines)
		log.Debugf("ingestFile sending %d lines", len(lines))
		out <- lines
	case "file_loop":
		// loop forever
		ticker := time.NewTicker(time.Duration(delaySeconds) * time.Second)
		for {
			select {
			case <-ingestF.exitChan:
				log.Debugf("exiting ingestFile because of signal")
				return
			case <-ticker.C:
				ingestF.PrevRecords = lines
				ingestF.TotalRecords += len(lines)
				log.Debugf("ingestFile sending %d lines", len(lines))
				out <- lines
			}
		}
	case "file_chunks":
		// sends the lines in chunks. Useful for testing parallelization
		ingestF.TotalRecords = len(lines)
		for len(lines) > 0 {
			if len(lines) > chunkLines {
				out <- lines[:chunkLines]
				lines = lines[chunkLines:]
			} else {
				out <- lines
				lines = nil
			}
		}
	}
}

func FileProvider(cfg config.File) node.StartFunc[[]interface{}] {
	return IngestFile{
		exitChan: utils.ExitChannel(),
		params:   cfg,
	}.Ingest
}
