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

package write

import (
	"fmt"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	log "github.com/sirupsen/logrus"
	"time"
)

type writeStdout struct {
}

// Write writes a flow before being stored
func (t *writeStdout) Write(in []config.GenericMap) []config.GenericMap {
	log.Debugf("entering writeStdout Write")
	log.Debugf("writeStdout: number of entries = %d", len(in))
	for _, v := range in {
		fmt.Printf("%s: %v\n", time.Now().Format(time.StampMilli), v)
	}

	return in
}

// NewWriteStdout create a new write
func NewWriteStdout() (Writer, error) {
	log.Debugf("entering NewWriteStdout")
	return &writeStdout{}, nil
}
