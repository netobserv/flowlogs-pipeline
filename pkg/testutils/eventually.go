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

package testutils

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Eventually retries a test until it eventually succeeds. If the timeout is reached, the test fails
// with the same failure as its last execution.
func Eventually(t *testing.T, timeout time.Duration, testFunc func(_ require.TestingT)) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	success := make(chan interface{})
	errorCh := make(chan error)
	failCh := make(chan error)

	go func() {
		for ctx.Err() == nil {
			result := testResult{failed: false, errorCh: errorCh, failCh: failCh}
			// Executing the function to test
			testFunc(&result)
			// If the function didn't reported failure and didn't reached timeout
			if !result.HasFailed() && ctx.Err() == nil {
				success <- 1
				break
			}
		}
	}()

	// Wait for success or timeout
	var err, fail error
	for {
		select {
		case <-success:
			return
		case err = <-errorCh:
		case fail = <-failCh:
		case <-ctx.Done():
			if err != nil {
				t.Error(err)
			} else if fail != nil {
				t.Error(fail)
			} else {
				t.Error("timeout while waiting for test to complete")
			}
			return
		}
	}
}

// util class for Eventually
type testResult struct {
	sync.RWMutex
	failed  bool
	errorCh chan<- error
	failCh  chan<- error
}

func (te *testResult) Errorf(format string, args ...interface{}) {
	te.Lock()
	te.failed = true
	te.Unlock()
	te.errorCh <- fmt.Errorf(format, args...)
}

func (te *testResult) FailNow() {
	te.Lock()
	te.failed = true
	te.Unlock()
	te.failCh <- errors.New("test failed")
}

func (te *testResult) HasFailed() bool {
	te.RLock()
	defer te.RUnlock()
	return te.failed
}
