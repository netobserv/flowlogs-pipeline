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

package utils

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SetupElegantExit(t *testing.T) {
	SetupElegantExit()

	select {
	case <-ExitChannel():
		// should not get here
		require.Error(t, fmt.Errorf("channel should have been empty"))
	default:
		break
	}

	// send signal and see that it is propagated
	err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	require.Equal(t, nil, err)

	select {
	case <-ExitChannel():
		break
	default:
		// should not get here
		require.Error(t, fmt.Errorf("channel should not be empty"))
		break
	}
}
