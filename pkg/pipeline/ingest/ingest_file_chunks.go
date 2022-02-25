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

	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
)

const chunkLines = 100

// FileChunks ingest entries from a file and resends them in chunks of fixed number of lines.
// It might be used to test processing speed in pipelines.
type FileChunks struct {
	fileName     string
	PrevRecords  []interface{}
	TotalRecords int
}

func (r *FileChunks) Ingest(out chan<- []interface{}) {
	lines := make([]interface{}, 0, chunkLines)
	file, err := os.Open(r.fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	nLines := 0
	for scanner.Scan() {
		text := scanner.Text()
		lines = append(lines, text)
		nLines++
		if nLines%chunkLines == 0 {
			r.PrevRecords = lines
			r.TotalRecords += len(lines)
			out <- lines
			// reset slice length without deallocating/reallocating memory
			lines = lines[:0]
		}
	}
	if len(lines) > 0 {
		r.PrevRecords = lines
		r.TotalRecords += len(lines)
		out <- lines
	}
}

// NewFileChunks create a new ingester that sends entries in chunks of fixed number of lines.
func NewFileChunks() (Ingester, error) {
	log.Debugf("entering NewIngestFile")
	if config.Opt.PipeLine.Ingest.File.Filename == "" {
		return nil, fmt.Errorf("ingest filename not specified")
	}

	log.Infof("input file name = %s", config.Opt.PipeLine.Ingest.File.Filename)

	ch := make(chan bool, 1)
	utils.RegisterExitChannel(ch)
	return &FileChunks{
		fileName: config.Opt.PipeLine.Ingest.File.Filename,
	}, nil
}
