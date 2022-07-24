/*
 * Copyright (C) 2022 IBM, Inc.
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
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

type WriteFake struct {
	AllRecords []config.GenericMap
	wait       chan struct{}
}

// Write stores in memory all records.
func (w *WriteFake) Write(in []config.GenericMap) {
	log.Debugf("entering writeFake Write")
	log.Debugf("writeFake: number of entries = %d", len(in))
	for _, r := range in {
		w.AllRecords = append(w.AllRecords, r.Copy())
	}
	close(w.wait)
}

// Wait waits until Write() is done processing all the records.
func (w *WriteFake) Wait() {
	<-w.wait
}

// ResetWait resets the wait channel to allow waiters to block on Wait() for the next Write().
// It should be invoked after Write() is done and all waiters are released.
func (w *WriteFake) ResetWait() {
	w.wait = make(chan struct{})
}

// NewWriteFake creates a new write.
func NewWriteFake(params config.StageParam) (Writer, error) {
	log.Debugf("entering NewWriteFake")
	w := &WriteFake{}
	w.ResetWait()
	return w, nil
}
