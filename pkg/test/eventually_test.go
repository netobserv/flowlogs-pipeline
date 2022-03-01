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

package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventually_Error(t *testing.T) {
	innerTest := &testing.T{}
	Eventually(innerTest, 10*time.Millisecond, func(t require.TestingT) {
		require.True(t, false)
	})
	assert.True(t, innerTest.Failed())
}

func TestEventually_Fail(t *testing.T) {
	innerTest := &testing.T{}
	Eventually(innerTest, 10*time.Millisecond, func(t require.TestingT) {
		t.FailNow()
	})
	assert.True(t, innerTest.Failed())
}

func TestEventually_Timeout(t *testing.T) {
	innerTest := &testing.T{}
	Eventually(innerTest, 10*time.Millisecond, func(t require.TestingT) {
		time.Sleep(5 * time.Second)
	})
	assert.True(t, innerTest.Failed())
}

func TestEventually_Success(t *testing.T) {
	num := 3
	Eventually(t, 5*time.Second, func(t require.TestingT) {
		require.Equal(t, 0, num)
		num--
	})
}
