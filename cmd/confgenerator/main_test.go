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

package main

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestTheMain(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestTheMain")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()
	var castErr *exec.ExitError
	if errors.As(err, &castErr) && !castErr.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}
